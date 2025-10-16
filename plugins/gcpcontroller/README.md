# GCP Controller Plugin

The GCP controller plugin automatically manages Google Cloud Platform (GCP) Compute Engine instances based on player activity, starting servers when players connect and shutting them down during idle periods.

## Features

- **Automatic Server Startup**: Starts the GCP instance when a player attempts to connect
- **Idle Shutdown**: Automatically stops the instance after a configurable idle timeout
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
- **idleTimeoutMinutes**: How long to wait after the last player disconnects before stopping the instance
- **startupThresholdMinutes**: Minimum time between server start attempts to prevent rapid restarts
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
3. **Player Tracking**: Once the server is running and players connect, their count is tracked
4. **Idle Detection**: When the last player disconnects, a shutdown timer begins
5. **Automatic Shutdown**: After the idle timeout expires with no players, the instance is stopped to save costs
6. **Shutdown Prevention**: If players rejoin during the idle timeout, the shutdown is cancelled
