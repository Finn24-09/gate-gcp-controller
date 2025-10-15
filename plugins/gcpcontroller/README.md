# GCP Controller Plugin for Gate Proxy

This plugin automatically manages GCP Compute Engine instances based on player activity, starting servers when players connect and stopping them after a period of inactivity.

## Features

- **Auto-Start**: Automatically starts your GCP backend server when a player tries to connect
- **Auto-Shutdown**: Stops the server after a configurable idle timeout (default: 30 minutes)
- **Startup Threshold**: Prevents duplicate start requests within a configurable cooldown period (default: 5 minutes)
- **Configurable Messages**: Customize the message shown to players when the server is starting

## Configuration

The plugin is configured using environment variables. All configuration must be set before starting the Gate proxy.

### Required Environment Variables

| Variable                        | Description                                                       | Example                |
| ------------------------------- | ----------------------------------------------------------------- | ---------------------- |
| `GCP_CONTROLLER_PROJECT_ID`     | Your GCP project ID                                               | `my-minecraft-project` |
| `GCP_CONTROLLER_ZONE`           | The zone where your instance is located                           | `us-central1-a`        |
| `GCP_CONTROLLER_INSTANCE_NAME`  | The name of your GCP Compute Engine instance                      | `minecraft-server`     |
| `GCP_CONTROLLER_SERVER_ADDRESS` | The server name from config.yml that this instance corresponds to | `server1`              |

### Optional Environment Variables

| Variable                                   | Description                                  | Default                                                           |
| ------------------------------------------ | -------------------------------------------- | ----------------------------------------------------------------- |
| `GCP_CONTROLLER_CREDENTIALS_PATH`          | Path to GCP service account JSON key file    | Uses Application Default Credentials                              |
| `GCP_CONTROLLER_IDLE_TIMEOUT_MINUTES`      | Minutes of inactivity before server shutdown | `30`                                                              |
| `GCP_CONTROLLER_STARTUP_THRESHOLD_MINUTES` | Cooldown period after triggering a start     | `5`                                                               |
| `GCP_CONTROLLER_STARTING_MESSAGE`          | Message shown when server is starting        | `Server is starting up! Please wait 30-60 seconds and try again.` |

## Setup Instructions

### 1. GCP Permissions

Your GCP credentials must have the following permissions:

- `compute.instances.get`
- `compute.instances.start`
- `compute.instances.stop`
- `compute.zoneOperations.get` (for waiting on operations)

You can use the `Compute Instance Admin (v1)` role or create a custom role with these specific permissions.

### 2. Authentication

**Option A: Service Account Key File (Recommended for production)**

1. Create a service account in GCP Console
2. Grant it the necessary permissions
3. Download the JSON key file
4. Set `GCP_CONTROLLER_CREDENTIALS_PATH` to the path of this file

**Option B: Application Default Credentials**
If you're running on GCP Compute Engine, you can use the instance's service account. Just ensure the instance has the necessary permissions and don't set `GCP_CONTROLLER_CREDENTIALS_PATH`.

### 3. Gate Configuration

Update your `config.yml` to define the backend server:

```yaml
config:
  bind: 0.0.0.0:25565
  onlineMode: true
  servers:
    server1: your-minecraft-server:25565 # This should match your GCP instance's address
  try:
    - server1
```

### 4. Set Environment Variables

**Linux/macOS:**

```bash
export GCP_CONTROLLER_PROJECT_ID="my-minecraft-project"
export GCP_CONTROLLER_ZONE="us-central1-a"
export GCP_CONTROLLER_INSTANCE_NAME="minecraft-server"
export GCP_CONTROLLER_SERVER_ADDRESS="server1"
export GCP_CONTROLLER_CREDENTIALS_PATH="/path/to/service-account-key.json"
export GCP_CONTROLLER_IDLE_TIMEOUT_MINUTES="30"
export GCP_CONTROLLER_STARTUP_THRESHOLD_MINUTES="5"
```

**Windows (PowerShell):**

```powershell
$env:GCP_CONTROLLER_PROJECT_ID="my-minecraft-project"
$env:GCP_CONTROLLER_ZONE="us-central1-a"
$env:GCP_CONTROLLER_INSTANCE_NAME="minecraft-server"
$env:GCP_CONTROLLER_SERVER_ADDRESS="server1"
$env:GCP_CONTROLLER_CREDENTIALS_PATH="C:\path\to\service-account-key.json"
$env:GCP_CONTROLLER_IDLE_TIMEOUT_MINUTES="30"
$env:GCP_CONTROLLER_STARTUP_THRESHOLD_MINUTES="5"
```

**Using a .env file (with tools like docker-compose):**

```env
GCP_CONTROLLER_PROJECT_ID=my-minecraft-project
GCP_CONTROLLER_ZONE=us-central1-a
GCP_CONTROLLER_INSTANCE_NAME=minecraft-server
GCP_CONTROLLER_SERVER_ADDRESS=server1
GCP_CONTROLLER_CREDENTIALS_PATH=/path/to/service-account-key.json
GCP_CONTROLLER_IDLE_TIMEOUT_MINUTES=30
GCP_CONTROLLER_STARTUP_THRESHOLD_MINUTES=5
```

### 5. Run Gate

```bash
go run .
```

## How It Works

### Player Connection Flow

1. **Player attempts to connect** to the proxy
2. **Plugin checks** if the target server is the managed GCP server
3. **If server is unreachable:**
   - Plugin checks if within startup threshold (5 min default)
   - If outside threshold, sends GCP API request to start instance
   - Player is kicked with configured message
   - Player should retry connection in 30-60 seconds
4. **If server is reachable:**
   - Player connects normally
   - Shutdown timer is cancelled if running

### Idle Shutdown Flow

1. **Last player disconnects** from managed server
2. **Plugin starts idle timer** (30 min default)
3. **After timeout expires:**
   - Plugin verifies no players have joined
   - Sends GCP API request to stop instance
   - Instance is stopped to save costs

### Startup Threshold

The startup threshold prevents multiple start requests when several players try to join simultaneously or in quick succession:

- After triggering a start, subsequent attempts within the threshold period (5 min default) will skip the API call
- Players still receive the "starting" message and are kicked
- This prevents API rate limiting and unnecessary requests

## Troubleshooting

### Plugin fails to initialize

**Error: `GCP_CONTROLLER_PROJECT_ID environment variable is required`**

- Ensure all required environment variables are set before starting Gate

**Error: `failed to create GCP compute client`**

- Check your credentials path is correct
- Verify the service account has necessary permissions
- Try using `gcloud auth application-default login` for local development

### Server doesn't start

**Check the logs for:**

- API permission errors - ensure service account has correct permissions
- Invalid project ID, zone, or instance name
- Instance already running/starting - check GCP console

### Server doesn't stop

- Verify the idle timeout is working by checking logs
- Ensure no players are actually connected
- Check GCP console to see current instance state

### Enable debug logging

Run Gate with the `-d` flag for verbose logging:

```bash
go run . -d
```

## Cost Optimization

This plugin helps reduce GCP costs by:

- Only running your Minecraft server when players are online
- Automatically stopping instances after inactivity
- Preventing unnecessary start/stop cycles with the startup threshold

**Estimated savings:** If players are only online 6 hours per day, this plugin can reduce your compute costs by approximately 75%.

## Development

### Building

```bash
go build .
```

### Testing

To test the plugin, you can:

1. Set up a test GCP instance
2. Configure environment variables
3. Connect with a Minecraft client
4. Verify the instance starts
5. Disconnect and wait for the idle timeout
6. Verify the instance stops

## License

This plugin follows the same license as the gate-plugin-template repository.
