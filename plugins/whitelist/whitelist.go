package whitelist

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/robinbraemer/event"
	"github.com/spf13/viper"
	"go.minekube.com/brigodier"
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is the whitelist plugin that manages player access control
var Plugin = proxy.Plugin{
	Name: "Whitelist",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Initializing Whitelist plugin")

		// Load configuration
		config, err := loadConfig()
		if err != nil {
			return fmt.Errorf("failed to load whitelist config: %w", err)
		}

		// Create whitelist manager
		manager := &whitelistManager{
			proxy:    p,
			config:   config,
			log:      log,
			entries:  make(map[string]*WhitelistEntry),
			filePath: config.WhitelistFile,
		}

		// Load whitelist from file
		if err := manager.load(); err != nil {
			log.Error(err, "Failed to load whitelist, starting with empty whitelist")
		}

		// Subscribe to connection events with high priority (higher value runs before other plugins)
		event.Subscribe(p.Event(), 100, manager.onPreConnect)

		// Register commands if enabled
		if config.Enabled {
			p.Command().Register(manager.whitelistCommand())
		}

		log.Info("Whitelist plugin initialized successfully",
			"enabled", config.Enabled,
			"entries", len(manager.entries))

		return nil
	},
}

// Config holds the whitelist configuration
type Config struct {
	Enabled       bool
	KickMessage   string
	WhitelistFile string
	Operators     []string // List of operator UUIDs
}

// WhitelistEntry represents a whitelisted player (matches Minecraft's format)
type WhitelistEntry struct {
	UUID string `json:"uuid"`
	Name string `json:"name"`
}

type whitelistManager struct {
	proxy    *proxy.Proxy
	config   *Config
	log      logr.Logger
	filePath string

	mu      sync.RWMutex
	entries map[string]*WhitelistEntry // UUID -> Entry
}

// loadConfig loads the whitelist configuration from config.yml
func loadConfig() (*Config, error) {
	cfg := &Config{
		Enabled:       true,
		KickMessage:   "You are not whitelisted on this server!",
		WhitelistFile: "whitelist.json",
	}

	// Create a new viper instance to read the config.yml file
	v := viper.New()
	v.SetConfigName("config")
	v.SetConfigType("yaml")
	v.AddConfigPath(".")
	v.AddConfigPath("/")

	if err := v.ReadInConfig(); err != nil {
		return cfg, nil // Return defaults if config doesn't exist
	}

	// Load whitelist settings from config.yml
	if v.IsSet("whitelist.enabled") {
		cfg.Enabled = v.GetBool("whitelist.enabled")
	}
	if v.IsSet("whitelist.kickMessage") {
		cfg.KickMessage = v.GetString("whitelist.kickMessage")
	}
	if v.IsSet("whitelist.whitelistFile") {
		cfg.WhitelistFile = v.GetString("whitelist.whitelistFile")
	}
	if v.IsSet("whitelist.operators") {
		cfg.Operators = v.GetStringSlice("whitelist.operators")
	}

	return cfg, nil
}

// onPreConnect handles player connection attempts before they reach other plugins
func (w *whitelistManager) onPreConnect(e *proxy.ServerPreConnectEvent) {
	// If whitelist is disabled, allow all connections
	if !w.config.Enabled {
		return
	}

	player := e.Player()
	uuid := player.ID().String()

	// Check if player is whitelisted
	if !w.isWhitelisted(uuid) {
		w.log.Info("Blocking non-whitelisted player",
			"player", player.Username(),
			"uuid", uuid)

		// Deny the connection
		e.Deny()

		// Kick player with message
		player.Disconnect(&c.Text{
			Content: w.config.KickMessage,
			S:       c.Style{Color: color.Red},
		})
		return
	}

	w.log.V(1).Info("Allowing whitelisted player",
		"player", player.Username(),
		"uuid", uuid)
}

// isWhitelisted checks if a UUID is in the whitelist
func (w *whitelistManager) isWhitelisted(uuid string) bool {
	w.mu.RLock()
	defer w.mu.RUnlock()
	_, exists := w.entries[uuid]
	return exists
}

// isOperator checks if a player UUID is in the operators list
func (w *whitelistManager) isOperator(uuid string) bool {
	for _, opUUID := range w.config.Operators {
		if opUUID == uuid {
			return true
		}
	}
	return false
}

// MojangProfile represents a Mojang API profile response
type MojangProfile struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// lookupPlayerUUID fetches a player's UUID from the Mojang API
func (w *whitelistManager) lookupPlayerUUID(username string) (string, string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}

	// Call Mojang API
	url := fmt.Sprintf("https://api.mojang.com/users/profiles/minecraft/%s", username)
	resp, err := client.Get(url)
	if err != nil {
		return "", "", fmt.Errorf("failed to contact Mojang API: %w", err)
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode == 404 {
		return "", "", fmt.Errorf("player not found")
	}
	if resp.StatusCode != 200 {
		return "", "", fmt.Errorf("mojang API returned status %d", resp.StatusCode)
	}

	// Parse response
	var profile MojangProfile
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return "", "", fmt.Errorf("failed to parse Mojang API response: %w", err)
	}

	// Format UUID with dashes (Mojang API returns without dashes)
	uuid := formatUUID(profile.ID)

	return uuid, profile.Name, nil
}

// formatUUID adds dashes to a UUID string without dashes
func formatUUID(uuidWithoutDashes string) string {
	if len(uuidWithoutDashes) != 32 {
		return uuidWithoutDashes
	}
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		uuidWithoutDashes[0:8],
		uuidWithoutDashes[8:12],
		uuidWithoutDashes[12:16],
		uuidWithoutDashes[16:20],
		uuidWithoutDashes[20:32])
}

// add adds a player to the whitelist
func (w *whitelistManager) add(uuid, username string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.entries[uuid] = &WhitelistEntry{
		UUID: uuid,
		Name: username,
	}

	return w.save()
}

// remove removes a player from the whitelist
func (w *whitelistManager) remove(uuid string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	delete(w.entries, uuid)
	return w.save()
}

// list returns all whitelisted players
func (w *whitelistManager) list() []*WhitelistEntry {
	w.mu.RLock()
	defer w.mu.RUnlock()

	entries := make([]*WhitelistEntry, 0, len(w.entries))
	for _, entry := range w.entries {
		entries = append(entries, entry)
	}
	return entries
}

// load loads the whitelist from file
func (w *whitelistManager) load() error {
	data, err := os.ReadFile(w.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, start with empty whitelist
			w.log.Info("Whitelist file not found, starting with empty whitelist",
				"path", w.filePath)
			return w.save() // Create empty file
		}
		return fmt.Errorf("failed to read whitelist file: %w", err)
	}

	var entries []*WhitelistEntry
	if err := json.Unmarshal(data, &entries); err != nil {
		return fmt.Errorf("failed to parse whitelist file: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	// Convert array to map for faster lookups
	w.entries = make(map[string]*WhitelistEntry)
	for _, entry := range entries {
		w.entries[entry.UUID] = entry
	}

	w.log.Info("Loaded whitelist from file",
		"path", w.filePath,
		"entries", len(w.entries))

	return nil
}

// save saves the whitelist to file
func (w *whitelistManager) save() error {
	// Convert map to array for JSON serialization
	entries := make([]*WhitelistEntry, 0, len(w.entries))
	for _, entry := range w.entries {
		entries = append(entries, entry)
	}

	data, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal whitelist: %w", err)
	}

	if err := os.WriteFile(w.filePath, data, 0644); err != nil {
		return fmt.Errorf("failed to write whitelist file: %w", err)
	}

	w.log.V(1).Info("Saved whitelist to file",
		"path", w.filePath,
		"entries", len(entries))

	return nil
}

// whitelistCommand creates the /whitelist command
func (w *whitelistManager) whitelistCommand() brigodier.LiteralNodeBuilder {
	return brigodier.Literal("whitelist").
		Then(w.whitelistAddCommand()).
		Then(w.whitelistRemoveCommand()).
		Then(w.whitelistListCommand())
}

// whitelistAddCommand creates the /whitelist add subcommand
func (w *whitelistManager) whitelistAddCommand() brigodier.LiteralNodeBuilder {
	return brigodier.Literal("add").
		Then(brigodier.Argument("player", brigodier.String).
			Executes(command.Command(func(ctx *command.Context) error {
				// Check if source is a player
				player, ok := ctx.Source.(proxy.Player)
				if !ok {
					return ctx.Source.SendMessage(&c.Text{
						Content: "This command can only be executed by players.",
						S:       c.Style{Color: color.Red},
					})
				}

				// Check if player is an operator
				if !w.isOperator(player.ID().String()) {
					return player.SendMessage(&c.Text{
						Content: "You're not permitted to use this command.",
						S:       c.Style{Color: color.Red},
					})
				}

				// Get target player name
				targetName := ctx.String("player")

				// Try to find online player first
				var targetUUID string
				var targetUsername string
				var foundOnline bool
				for _, p := range w.proxy.Players() {
					if p.Username() == targetName {
						targetUUID = p.ID().String()
						targetUsername = p.Username()
						foundOnline = true
						break
					}
				}

				// If player not online, lookup via Mojang API
				if !foundOnline {
					w.log.Info("Player not online, looking up via Mojang API",
						"username", targetName)

					uuid, username, err := w.lookupPlayerUUID(targetName)
					if err != nil {
						if err.Error() == "player not found" {
							return player.SendMessage(&c.Text{
								Content: fmt.Sprintf("Player '%s' does not exist.", targetName),
								S:       c.Style{Color: color.Red},
							})
						}
						w.log.Error(err, "Failed to lookup player UUID",
							"username", targetName)
						return player.SendMessage(&c.Text{
							Content: fmt.Sprintf("Failed to lookup player '%s'. Error: %s\nYou can manually edit whitelist.json if needed.", targetName, err.Error()),
							S:       c.Style{Color: color.Red},
						})
					}
					targetUUID = uuid
					targetUsername = username
				}

				// Add to whitelist
				if err := w.add(targetUUID, targetUsername); err != nil {
					w.log.Error(err, "Failed to add player to whitelist",
						"player", targetName)
					return player.SendMessage(&c.Text{
						Content: fmt.Sprintf("Failed to add %s to the whitelist.", targetName),
						S:       c.Style{Color: color.Red},
					})
				}

				w.log.Info("Player added to whitelist",
					"target", targetName,
					"addedBy", player.Username())

				return player.SendMessage(&c.Text{
					Content: fmt.Sprintf("Added %s to the whitelist.", targetName),
					S:       c.Style{Color: color.Green},
				})
			})))
}

// whitelistRemoveCommand creates the /whitelist remove subcommand
func (w *whitelistManager) whitelistRemoveCommand() brigodier.LiteralNodeBuilder {
	return brigodier.Literal("remove").
		Then(brigodier.Argument("player", brigodier.String).
			Executes(command.Command(func(ctx *command.Context) error {
				// Check if source is a player
				player, ok := ctx.Source.(proxy.Player)
				if !ok {
					return ctx.Source.SendMessage(&c.Text{
						Content: "This command can only be executed by players.",
						S:       c.Style{Color: color.Red},
					})
				}

				// Check if player is an operator
				if !w.isOperator(player.ID().String()) {
					return player.SendMessage(&c.Text{
						Content: "You're not permitted to use this command.",
						S:       c.Style{Color: color.Red},
					})
				}

				// Get target player name
				targetName := ctx.String("player")

				// Find player in whitelist
				var targetUUID string
				w.mu.RLock()
				for uuid, entry := range w.entries {
					if entry.Name == targetName {
						targetUUID = uuid
						break
					}
				}
				w.mu.RUnlock()

				if targetUUID == "" {
					return player.SendMessage(&c.Text{
						Content: fmt.Sprintf("Player '%s' is not in the whitelist.", targetName),
						S:       c.Style{Color: color.Red},
					})
				}

				// Remove from whitelist
				if err := w.remove(targetUUID); err != nil {
					w.log.Error(err, "Failed to remove player from whitelist",
						"player", targetName)
					return player.SendMessage(&c.Text{
						Content: fmt.Sprintf("Failed to remove %s from the whitelist.", targetName),
						S:       c.Style{Color: color.Red},
					})
				}

				w.log.Info("Player removed from whitelist",
					"target", targetName,
					"removedBy", player.Username())

				return player.SendMessage(&c.Text{
					Content: fmt.Sprintf("Removed %s from the whitelist.", targetName),
					S:       c.Style{Color: color.Green},
				})
			})))
}

// whitelistListCommand creates the /whitelist list subcommand
func (w *whitelistManager) whitelistListCommand() brigodier.LiteralNodeBuilder {
	return brigodier.Literal("list").
		Executes(command.Command(func(ctx *command.Context) error {
			// Check if source is a player
			player, ok := ctx.Source.(proxy.Player)
			if !ok {
				return ctx.Source.SendMessage(&c.Text{
					Content: "This command can only be executed by players.",
					S:       c.Style{Color: color.Red},
				})
			}

			// Check if player is an operator
			if !w.isOperator(player.ID().String()) {
				return player.SendMessage(&c.Text{
					Content: "You're not permitted to use this command.",
					S:       c.Style{Color: color.Red},
				})
			}

			entries := w.list()

			if len(entries) == 0 {
				return player.SendMessage(&c.Text{
					Content: "The whitelist is empty.",
					S:       c.Style{Color: color.Yellow},
				})
			}

			// Build response
			header := &c.Text{
				Content: fmt.Sprintf("Whitelisted players (%d):", len(entries)),
				S:       c.Style{Color: color.Gold, Bold: c.True},
			}

			message := &c.Text{}
			message.Extra = []c.Component{header}

			for _, entry := range entries {
				line := &c.Text{
					Content: fmt.Sprintf("\n  - %s (%s)", entry.Name, entry.UUID),
					S:       c.Style{Color: color.White},
				}
				message.Extra = append(message.Extra, line)
			}

			return player.SendMessage(message)
		}))
}
