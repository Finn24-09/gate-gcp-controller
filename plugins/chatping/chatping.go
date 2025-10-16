package chatping

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/go-logr/logr"
	"github.com/minekube/gate-plugin-template/util"
	"go.minekube.com/brigodier"
	"go.minekube.com/common/minecraft/color"
	c "go.minekube.com/common/minecraft/component"
	"go.minekube.com/gate/pkg/command"
	"go.minekube.com/gate/pkg/edition/java/proxy"
)

// Plugin is a chat ping plugin that allows players to check their ping.
var Plugin = proxy.Plugin{
	Name: "ChatPing",
	Init: func(ctx context.Context, p *proxy.Proxy) error {
		log := logr.FromContextOrDiscard(ctx)
		log.Info("Hello from ChatPing plugin!")

		pl := &plugin{
			cooldowns: make(map[string]time.Time),
		}

		p.Command().Register(pl.pingCommand())

		return nil
	},
}

type plugin struct {
	cooldowns map[string]time.Time
	mu        sync.RWMutex
}

const cooldownDuration = 30 * time.Second

func (p *plugin) pingCommand() brigodier.LiteralNodeBuilder {
	pingCmd := command.Command(func(ctx *command.Context) error {
		player, ok := ctx.Source.(proxy.Player)
		if !ok {
			return ctx.Source.SendMessage(&c.Text{Content: "You must be a player to run this command."})
		}

		playerID := player.ID().String()

		// Check cooldown
		p.mu.RLock()
		lastUsed, exists := p.cooldowns[playerID]
		p.mu.RUnlock()

		if exists {
			elapsed := time.Since(lastUsed)
			if elapsed < cooldownDuration {
				remaining := cooldownDuration - elapsed
				remainingSeconds := int(remaining.Seconds()) + 1

				cooldownMsg := &c.Text{
					Content: fmt.Sprintf("Please wait %d seconds before using /ping again.", remainingSeconds),
					S:       c.Style{Color: color.Red},
				}
				return player.SendMessage(cooldownMsg)
			}
		}

		// Update cooldown
		p.mu.Lock()
		p.cooldowns[playerID] = time.Now()
		p.mu.Unlock()

		// Get player ping
		ping := player.Ping().Milliseconds()

		// Determine connection health and color
		health, healthColor := getConnectionHealth(ping)

		// Create ping message
		pingMsg := createPingMessage(ping, health, healthColor)
		return player.SendMessage(pingMsg)
	})

	return brigodier.Literal("ping").Executes(pingCmd)
}

func getConnectionHealth(pingMs int64) (string, *color.RGB) {
	switch {
	case pingMs < 50:
		return "Excellent", color.Green.RGB
	case pingMs < 100:
		return "Good", color.Yellow.RGB
	case pingMs < 150:
		return "Fair", color.Gold.RGB
	case pingMs < 250:
		return "Poor", color.Red.RGB
	default:
		return "Bad", color.DarkRed.RGB
	}
}

func createPingMessage(ping int64, health string, healthColor *color.RGB) c.Component {
	prefix := &c.Text{
		Content: "Your ping: ",
		S:       c.Style{Color: color.Gray},
	}

	pingValue := &c.Text{
		Content: fmt.Sprintf("%dms", ping),
		S:       c.Style{Color: color.White, Bold: c.True},
	}

	separator := &c.Text{
		Content: " - ",
		S:       c.Style{Color: color.Gray},
	}

	healthStatus := &c.Text{
		Content: health,
		S:       c.Style{Color: healthColor, Bold: c.True},
	}

	return util.Join(prefix, pingValue, separator, healthStatus)
}
