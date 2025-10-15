package gcpcontroller

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"github.com/spf13/viper"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/edition/java/proxy"
	"google.golang.org/api/option"
)

// Plugin is the GCP controller plugin that manages backend server lifecycle
var Plugin = proxy.Plugin{
	Name: "GCPController",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Initializing GCP Controller plugin")

		// Load configuration
		config, err := loadConfig(p)
		if err != nil {
			return fmt.Errorf("failed to load GCP controller config: %w", err)
		}

		// Create GCP client
		var clientOpts []option.ClientOption
		if config.CredentialsPath != "" {
			clientOpts = append(clientOpts, option.WithCredentialsFile(config.CredentialsPath))
		}

		client, err := compute.NewInstancesRESTClient(ctx, clientOpts...)
		if err != nil {
			return fmt.Errorf("failed to create GCP compute client: %w", err)
		}

		controller := &gcpController{
			proxy:         p,
			config:        config,
			client:        client,
			log:           log,
			playerCount:   0,
			lastActivity:  time.Now(),
			lastStartTime: time.Time{},
			shutdownTimer: nil,
		}

		// Subscribe to events
		event.Subscribe(p.Event(), 0, controller.onServerPreConnect)
		event.Subscribe(p.Event(), 0, controller.onServerPostConnect)
		event.Subscribe(p.Event(), 0, controller.onDisconnect)

		log.Info("GCP Controller plugin initialized successfully",
			"project", config.ProjectID,
			"zone", config.Zone,
			"instance", config.InstanceName)

		return nil
	},
}

type gcpController struct {
	proxy  *proxy.Proxy
	config *Config
	client *compute.InstancesClient
	log    logr.Logger

	mu            sync.RWMutex
	playerCount   int
	lastActivity  time.Time
	lastStartTime time.Time
	shutdownTimer *time.Timer
	isStarting    bool
}

// Config holds the GCP controller configuration
type Config struct {
	ProjectID               string
	Zone                    string
	InstanceName            string
	ServerAddress           string
	IdleTimeoutMinutes      int
	StartupThresholdMinutes int
	StartingMessage         string
	CredentialsPath         string
}

// loadConfig loads the GCP controller configuration from config.yml
func loadConfig(_ *proxy.Proxy) (*Config, error) {
	cfg := &Config{
		IdleTimeoutMinutes:      30,
		StartupThresholdMinutes: 5,
		StartingMessage:         "Server is starting up! Please wait 30-60 seconds and try again.",
	}

	// Create a new viper instance to read the config.yml file
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/")

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("failed to read config.yml: %w", err)
	}

	// Load GCP controller settings from config.yml
	if v.IsSet("gcpController.projectId") {
		cfg.ProjectID = v.GetString("gcpController.projectId")
	}
	if v.IsSet("gcpController.zone") {
		cfg.Zone = v.GetString("gcpController.zone")
	}
	if v.IsSet("gcpController.instanceName") {
		cfg.InstanceName = v.GetString("gcpController.instanceName")
	}
	if v.IsSet("gcpController.serverAddress") {
		cfg.ServerAddress = v.GetString("gcpController.serverAddress")
	}
	if v.IsSet("gcpController.credentialsPath") {
		cfg.CredentialsPath = v.GetString("gcpController.credentialsPath")
	}
	if v.IsSet("gcpController.idleTimeoutMinutes") {
		cfg.IdleTimeoutMinutes = v.GetInt("gcpController.idleTimeoutMinutes")
	}
	if v.IsSet("gcpController.startupThresholdMinutes") {
		cfg.StartupThresholdMinutes = v.GetInt("gcpController.startupThresholdMinutes")
	}
	if v.IsSet("gcpController.startingMessage") {
		cfg.StartingMessage = v.GetString("gcpController.startingMessage")
	}

	// Validate required fields
	if cfg.ProjectID == "" {
		return nil, fmt.Errorf("gcpController.projectId is required in config.yml")
	}
	if cfg.Zone == "" {
		return nil, fmt.Errorf("gcpController.zone is required in config.yml")
	}
	if cfg.InstanceName == "" {
		return nil, fmt.Errorf("gcpController.instanceName is required in config.yml")
	}
	if cfg.ServerAddress == "" {
		return nil, fmt.Errorf("gcpController.serverAddress is required in config.yml")
	}

	return cfg, nil
}

// onServerPreConnect handles player connection attempts before they connect to a server
func (g *gcpController) onServerPreConnect(e *proxy.ServerPreConnectEvent) {
	// Check if this is connecting to our managed server
	server := e.OriginalServer()
	if server == nil || server.ServerInfo().Name() != g.config.ServerAddress {
		return
	}

	// Check if server is reachable
	if g.isServerReachable(server) {
		g.log.V(1).Info("Server is reachable, allowing connection",
			"player", e.Player().Username(),
			"server", server.ServerInfo().Name())
		return
	}

	// Server is not reachable, attempt to start it
	g.log.Info("Server is not reachable, attempting to start GCP instance",
		"player", e.Player().Username(),
		"server", server.ServerInfo().Name())

	if err := g.tryStartServer(e.Player().Context()); err != nil {
		g.log.Error(err, "Failed to start GCP instance")
	}

	// Deny connection and kick player with message
	e.Deny()
	e.Player().Disconnect(&c.Text{
		Content: g.config.StartingMessage,
	})
}

// onServerPostConnect handles player successfully connecting to a server
func (g *gcpController) onServerPostConnect(e *proxy.ServerPostConnectEvent) {
	// Check if connected to our managed server
	if e.Player().CurrentServer() == nil ||
		e.Player().CurrentServer().Server().ServerInfo().Name() != g.config.ServerAddress {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.playerCount++
	g.lastActivity = time.Now()

	// Cancel shutdown timer if it's running
	if g.shutdownTimer != nil {
		g.shutdownTimer.Stop()
		g.shutdownTimer = nil
		g.log.Info("Cancelled scheduled server shutdown due to player join",
			"player", e.Player().Username(),
			"playerCount", g.playerCount)
	}

	g.log.V(1).Info("Player connected to managed server",
		"player", e.Player().Username(),
		"playerCount", g.playerCount)
}

// onDisconnect handles player disconnecting
func (g *gcpController) onDisconnect(e *proxy.DisconnectEvent) {
	player := e.Player()
	if player.CurrentServer() == nil ||
		player.CurrentServer().Server().ServerInfo().Name() != g.config.ServerAddress {
		return
	}

	g.mu.Lock()
	defer g.mu.Unlock()

	g.playerCount--
	if g.playerCount < 0 {
		g.playerCount = 0
	}

	g.lastActivity = time.Now()

	g.log.V(1).Info("Player disconnected from managed server",
		"player", player.Username(),
		"playerCount", g.playerCount)

	// If no players left, start shutdown timer
	if g.playerCount == 0 {
		g.scheduleShutdown()
	}
}

// isServerReachable checks if the server is currently reachable
func (g *gcpController) isServerReachable(server proxy.RegisteredServer) bool {
	// Try to connect to the server to check if it's reachable
	// We create a connection request and check if we can establish a connection
	addr := server.ServerInfo().Addr()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Try to establish a TCP connection
	dialer := &net.Dialer{}
	conn, err := dialer.DialContext(ctx, "tcp", addr.String())
	if err != nil {
		return false
	}
	defer conn.Close()

	return true
}

// tryStartServer attempts to start the GCP instance
func (g *gcpController) tryStartServer(ctx context.Context) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// Check startup threshold
	if !g.lastStartTime.IsZero() {
		threshold := time.Duration(g.config.StartupThresholdMinutes) * time.Minute
		if time.Since(g.lastStartTime) < threshold {
			g.log.Info("Within startup threshold, skipping start request",
				"lastStart", g.lastStartTime,
				"threshold", threshold)
			return nil
		}
	}

	// Check current instance state
	instance, err := g.client.Get(ctx, &computepb.GetInstanceRequest{
		Project:  g.config.ProjectID,
		Zone:     g.config.Zone,
		Instance: g.config.InstanceName,
	})
	if err != nil {
		return fmt.Errorf("failed to get instance state: %w", err)
	}

	status := instance.GetStatus()
	g.log.Info("Current instance status", "status", status)

	// Only start if instance is stopped
	if status != "TERMINATED" && status != "STOPPED" {
		g.log.Info("Instance is not stopped, skipping start",
			"status", status)
		return nil
	}

	// Start the instance
	g.log.Info("Starting GCP instance",
		"project", g.config.ProjectID,
		"zone", g.config.Zone,
		"instance", g.config.InstanceName)

	op, err := g.client.Start(ctx, &computepb.StartInstanceRequest{
		Project:  g.config.ProjectID,
		Zone:     g.config.Zone,
		Instance: g.config.InstanceName,
	})
	if err != nil {
		return fmt.Errorf("failed to start instance: %w", err)
	}

	// Wait for operation to complete (with timeout)
	waitCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()

	if err := op.Wait(waitCtx); err != nil {
		return fmt.Errorf("failed to wait for start operation: %w", err)
	}

	g.lastStartTime = time.Now()
	g.isStarting = true

	g.log.Info("Successfully started GCP instance")

	return nil
}

// scheduleShutdown schedules the server to shutdown after the idle timeout
func (g *gcpController) scheduleShutdown() {
	if g.shutdownTimer != nil {
		g.shutdownTimer.Stop()
	}

	timeout := time.Duration(g.config.IdleTimeoutMinutes) * time.Minute
	g.shutdownTimer = time.AfterFunc(timeout, func() {
		g.mu.Lock()
		defer g.mu.Unlock()

		// Double-check no players have joined
		if g.playerCount > 0 {
			g.log.Info("Players online, cancelling shutdown")
			return
		}

		g.log.Info("Idle timeout reached, shutting down GCP instance",
			"timeout", timeout)

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := g.stopServer(ctx); err != nil {
			g.log.Error(err, "Failed to shutdown GCP instance")
		}
	})

	g.log.Info("Scheduled server shutdown",
		"timeout", timeout,
		"shutdownAt", time.Now().Add(timeout))
}

// stopServer stops the GCP instance
func (g *gcpController) stopServer(ctx context.Context) error {
	// Check current instance state
	instance, err := g.client.Get(ctx, &computepb.GetInstanceRequest{
		Project:  g.config.ProjectID,
		Zone:     g.config.Zone,
		Instance: g.config.InstanceName,
	})
	if err != nil {
		return fmt.Errorf("failed to get instance state: %w", err)
	}

	status := instance.GetStatus()
	g.log.Info("Current instance status before stop", "status", status)

	// Only stop if instance is running
	if status != "RUNNING" {
		g.log.Info("Instance is not running, skipping stop",
			"status", status)
		return nil
	}

	// Stop the instance
	g.log.Info("Stopping GCP instance",
		"project", g.config.ProjectID,
		"zone", g.config.Zone,
		"instance", g.config.InstanceName)

	op, err := g.client.Stop(ctx, &computepb.StopInstanceRequest{
		Project:  g.config.ProjectID,
		Zone:     g.config.Zone,
		Instance: g.config.InstanceName,
	})
	if err != nil {
		return fmt.Errorf("failed to stop instance: %w", err)
	}

	// Wait for operation to complete
	if err := op.Wait(ctx); err != nil {
		return fmt.Errorf("failed to wait for stop operation: %w", err)
	}

	g.log.Info("Successfully stopped GCP instance")

	return nil
}
