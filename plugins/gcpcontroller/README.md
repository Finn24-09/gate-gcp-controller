# GCP Controller Plugin

The GCP controller plugin automatically manages Google Cloud Platform (GCP) Compute Engine instances based on player activity, starting servers when players connect and shutting them down during idle periods.

## Features

- **Automatic Server Startup**: Starts the GCP instance when a player attempts to connect
- **Idle Shutdown**: Automatically stops the instance after a configurable idle timeout
- **Safety Shutdown**: Shutting down if no one joins after startup
- **Startup Throttling**: Prevents repeated start attempts within a threshold period
- **Connection Health Monitoring**: Checks server reachability before allowing connections
- **Player Count Tracking**: Monitors active players to prevent premature shutdowns
- **Customizable Messages**: Configure the message shown to players during server startup
- **GCP Integration**: Uses official Google Cloud SDK for reliable instance management

## Commands

This plugin does not provide any player-facing commands. All operations are automatic based on player connections and disconnections.

## Configuration

Configure the plugin in your `config.yml` file under the `gcpController` section:

### Configuration Parameters

- **projectId**: Your Google Cloud Platform project ID
- **zone**: The GCP zone where your Compute Engine instance is located
- **instanceName**: The name of your Compute Engine instance
- **serverAddress**: The server name as configured in Gate's server list (must match)
- **credentialsPath**: Path to service account JSON credentials (optional if using ADC)
- **idleTimeoutMinutes**: How long to wait after the last player disconnects before stopping the instance (default: 30 minutes)
- **startupThresholdMinutes**: Minimum time between server start attempts to prevent rapid restarts (default: 5 minutes)
- **noJoinTimeoutMinutes**: Minutes to wait for a player to join after server startup before automatic shutdown (default: 15 minutes)
- **startingMessage**: Custom message displayed to players when the server is starting up

### GCP Permissions

The service account or Application Default Credentials must have the following IAM permissions:

- `compute.instances.get`
- `compute.instances.start`
- `compute.instances.stop`

These are typically provided by the `Editor` role or similar.

## How It Works

1. **Connection Attempt**: When a player tries to connect to the managed server, the plugin checks if it's reachable
2. **Server Starting**: If unreachable, the plugin starts the GCP instance and kicks the player with a startup message
3. **Safety Timer**: After starting, a safety timer begins (default: 15 minutes). If no player successfully joins within this time, the server is shut down to prevent unnecessary costs from abandoned startup attempts
4. **Player Tracking**: Once a player successfully connects, the safety timer is cancelled and their count is tracked
5. **Idle Detection**: When the last player disconnects, an idle shutdown timer begins (default: 30 minutes)
6. **Automatic Shutdown**: After the idle timeout expires with no players, the instance is stopped to save costs
7. **Shutdown Prevention**: If players rejoin during the idle timeout, the shutdown is cancelled

## Cost Optimization

The plugin includes two shutdown mechanisms to minimize GCP costs:

### Idle Shutdown (idleTimeoutMinutes)

Stops the server after players have been playing but all disconnect. This prevents the server from running indefinitely when no one is online.

### Safety Shutdown (noJoinTimeoutMinutes)

Prevents a scenario where:

- A player attempts to connect, triggering the server to start
- The player decides not to wait and disconnects from the proxy
- The server continues running indefinitely, incurring unnecessary costs

With the safety shutdown enabled, if no player successfully joins the game server within the configured timeout (default: 15 minutes), the instance is automatically shut down. This ensures you only pay for actual gameplay time.
