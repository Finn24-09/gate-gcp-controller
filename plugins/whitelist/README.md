# Whitelist Plugin

The whitelist plugin provides server access control by restricting connections to authorized players only, with support for both online and offline player management.

## Features

- **Access Control**: Restricts server access to whitelisted players when enabled
- **Runtime Adjustable**: Add/remove players without restarting the proxy
- **Minecraft Compatible**: Uses the official Minecraft whitelist.json format
- **Operator Only**: Only operators can manage the whitelist
- **Online & Offline Support**: Add players whether they're currently online or not
- **Mojang API Integration**: Automatically fetches UUIDs for offline players
- **Prevents Server Starts**: Non-whitelisted players won't trigger GCP instance startup
- **High Priority**: Runs before other plugins to block connections early
- **Persistent Storage**: Whitelist changes are automatically saved and persist across restarts

## Commands

Only operators (configured in `config.yml`) can use these commands:

```bash
/whitelist add <player>     # Add a player to the whitelist
/whitelist remove <player>  # Remove a player from the whitelist
/whitelist list             # Show all whitelisted players
```

**Notes:**

- The `/whitelist add` command works for both online and offline players
- If the player isn't currently online, their UUID is automatically fetched from Mojang's API
- All changes are immediately saved to `whitelist.json` and take effect instantly

## Configuration

Configure the plugin in your `config.yml` file under the `whitelist` section:

### Configuration Parameters

- **enabled**: Set to `true` to enforce whitelist, `false` to allow all players (default: `true`)
- **kickMessage**: Custom message displayed to non-whitelisted players when they're denied access
- **whitelistFile**: Path to the JSON file storing whitelisted players (default: `whitelist.json`)
- **operators**: Array of player UUIDs authorized to use whitelist commands

### Whitelist File Format

The [`whitelist.json`](/whitelist.json) file follows Minecraft's standard format.

This file is automatically managed by the plugin and shouldn't require manual editing.

## How It Works

1. **Connection Check**: When a player attempts to connect, the plugin checks if whitelist is enabled
2. **UUID Verification**: The player's UUID is verified against the whitelist entries
3. **Access Decision**: Whitelisted players are allowed through; others are kicked with the configured message
4. **Priority Processing**: Runs with high priority (100) to block non-whitelisted players before other plugins process them
5. **GCP Integration**: Non-whitelisted players won't trigger the GCP controller to start instances
