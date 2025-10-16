# ChatPing Plugin

The chatping plugin provides a simple way for players to check their connection latency with color-coded health indicators.

## Features

- **Ping Check**: Players can check their current ping to the server
- **Health Indicators**: Color-coded connection quality (Excellent, Good, Fair, Poor, Bad)
- **Cooldown Protection**: 30-second cooldown per player to prevent spam
- **Visual Feedback**: Formatted messages with colors and bold text for easy reading

## Commands

All players can use this command:

```bash
/ping    # Check your current ping with connection health status
```

**Connection Health Levels:**

- **Excellent** (Green): < 50ms
- **Good** (Yellow): 50-99ms
- **Fair** (Gold): 100-149ms
- **Poor** (Red): 150-249ms
- **Bad** (Dark Red): ≥ 250ms

**Note:** The command has a 30-second cooldown to prevent spam. If used before the cooldown expires, players will receive a message indicating how many seconds remain.

## Configuration

This plugin does not require any configuration in `config.yml`. It works out of the box once enabled.
