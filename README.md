<a name="readme-top"></a>

# Gate GCP Controller

## About

This project extends [Minekube Gate](https://github.com/minekube/gate) with a GCP Controller plugin that automatically manages your Minecraft backend servers on Google Cloud Platform.

### Features

✨ **Auto-Start**: Automatically starts your GCP instance when a player connects  
✨ **Auto-Shutdown**: Stops the server after configurable idle time (default: 30 minutes)  
✨ **Startup Threshold**: Prevents duplicate start requests (default: 5 minute cooldown)  
✨ **Flexible Auth**: Supports both credential files and Application Default Credentials  
✨ **Safety Shutdown**: Shutting down if no one joins after startup  
✨ **Docker Ready**: One-command deployment with Docker Compose  
✨ **[Whitelist Plugin](/plugins/whitelist/README.md)**: Restrict server access to authorized players only with runtime-adjustable whitelist  
✨ **[ChatPing Plugin](/plugins/chatping/README.md)**: Allows players to check their ping with `/ping` command

## Quick Start

### Prerequisites

- **GCP Account** with a Compute Engine instance running Minecraft
- **Docker** & Docker Compose V2 (for containerized deployment)
  - Note: Use `docker compose` (v2) instead of `docker-compose` (v1)
- **OR** Go 1.20+ (for local development)

### Option 1: Docker Deployment (Recommended)

1. **Prepare a working directory (any path is fine):**

   ```bash
   mkdir gate-gcp-controller && cd gate-gcp-controller
   ```

2. **Grab the config templates (or create your own):**

   ```bash
   curl -L https://raw.githubusercontent.com/Finn24-09/gate-gcp-controller/main/config.yml -o config.yml
   curl -L https://raw.githubusercontent.com/Finn24-09/gate-gcp-controller/main/whitelist.json -o whitelist.json
   ```

   Adjust `config.yml` for your project, and edit `whitelist.json` if you plan to enable the whitelist (defaults to `[]`).

3. **Set up credentials** (skip if using ADC):

   ```bash
   curl -L https://raw.githubusercontent.com/Finn24-09/gate-gcp-controller/main/.env.example -o .env
   # Edit .env and set: GCP_CREDENTIALS_FILE=/absolute/path/to/your-key.json
   ```

4. **Create `docker-compose.yml` using the published image:**

   ```bash
   curl -L https://raw.githubusercontent.com/Finn24-09/gate-gcp-controller/main/docker-compose.yml -o docker-compose.yml
   ```

5. **Deploy with Docker Compose:**

   ```bash
   docker compose up -d
   docker compose logs -f
   ```

### Option 2: Local Development

1. **Clone and build:**

   ```bash
   git clone https://github.com/Finn24-09/gate-gcp-controller.git
   cd gate-gcp-controller
   go mod download
   go build .
   ```

2. **Configure [`config.yml`](/config.yml)**

3. **Configure [`whitelist file`](/whitelist.json) if whitelist is enabled inside the config.yml**

4. **Run:**
   ```bash
   ./gate-gcp-controller
   ```

## Complete Setup Guide

### 1. GCP Configuration

#### Create a Service Account

```bash
# Create service account
gcloud iam service-accounts create minecraft-controller \
  --description="Controls Minecraft server instances" \
  --display-name="Minecraft Controller"

# Create custom role with minimal permissions
gcloud iam roles create MinecraftServerController \
  --project=YOUR_PROJECT_ID \
  --permissions=compute.instances.get,compute.instances.start,compute.instances.stop,compute.zoneOperations.get

# Grant role to service account
gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
  --member="serviceAccount:minecraft-controller@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
  --role="projects/YOUR_PROJECT_ID/roles/MinecraftServerController"

# Create and download key (skip if using ADC)
gcloud iam service-accounts keys create ~/minecraft-controller-key.json \
  --iam-account=minecraft-controller@YOUR_PROJECT_ID.iam.gserviceaccount.com
```

#### Set Up Firewall Rules

```bash
# Allow Minecraft connections to proxy
gcloud compute firewall-rules create minecraft-proxy \
  --allow=tcp:25565 \
  --source-ranges=0.0.0.0/0 \
  --target-tags=minecraft-proxy

# Allow proxy to connect to backend (optional but recommended)
gcloud compute firewall-rules create minecraft-backend-from-proxy \
  --allow=tcp:25565 \
  --source-tags=minecraft-proxy \
  --target-tags=minecraft-backend

# Tag your VMs
gcloud compute instances add-tags YOUR_PROXY_VM --tags=minecraft-proxy --zone=YOUR_ZONE
gcloud compute instances add-tags YOUR_MINECRAFT_VM --tags=minecraft-backend --zone=YOUR_ZONE
```

#### Using Application Default Credentials (ADC)

If running on GCP Compute Engine, you can skip the credentials file:

```bash
# Attach service account to your proxy VM (requires VM to be stopped)
gcloud compute instances stop YOUR_PROXY_VM --zone=YOUR_ZONE

gcloud compute instances set-service-account YOUR_PROXY_VM \
  --service-account=minecraft-controller@YOUR_PROJECT_ID.iam.gserviceaccount.com \
  --scopes=https://www.googleapis.com/auth/compute-engine \
  --zone=YOUR_ZONE

gcloud compute instances start YOUR_PROXY_VM --zone=YOUR_ZONE
```

Then comment out `credentialsPath` in `config.yml`:

```yaml
gcpController:
  # credentialsPath: "/credentials/gcp-key.json"  # Commented for ADC
```

### 2. Backend Minecraft Server Configuration

**CRITICAL**: Your backend Minecraft server MUST be configured to accept proxy connections.

#### For Spigot/Paper Servers

1. **Edit `server.properties`:**

   ```properties
   online-mode=false
   ```

2. **Edit `spigot.yml`:**

   ```yaml
   settings:
     bungeecord: true
   ```

3. **For Paper servers, edit `config/paper-global.yml`:**

   ```yaml
   proxies:
     bungee-cord:
       online-mode: true
     velocity:
       enabled: false # Set to true if using velocity mode
       online-mode: false
   ```

4. **Restart your Minecraft server** (full restart, NOT /reload)

#### Forwarding Modes

**Legacy Mode** (works with all servers):

```yaml
# Gate config.yml
config:
  forwarding:
    mode: legacy

# Backend spigot.yml
settings:
  bungeecord: true
```

**Velocity Mode** (Paper 1.13+, more secure):

```yaml
# Gate config.yml
config:
  forwarding:
    mode: velocity
    secret: "your-random-secret-here"

# Backend config/paper-global.yml
proxies:
  velocity:
    enabled: true
    online-mode: true
    secret: "your-random-secret-here" # Same as Gate
```

## Troubleshooting

### GCP Issues

#### "Error 403: Request had insufficient authentication scopes"

**Cause**: VM lacks authentication scopes

**Solution**:

```bash
# Stop VM
gcloud compute instances stop YOUR_VM --zone=YOUR_ZONE

# Set scopes
gcloud compute instances set-service-account YOUR_VM \
  --service-account=YOUR_SA@YOUR_PROJECT.iam.gserviceaccount.com \
  --scopes=https://www.googleapis.com/auth/cloud-platform \
  --zone=YOUR_ZONE

# Start VM
gcloud compute instances start YOUR_VM --zone=YOUR_ZONE
```

#### "Failed to start GCP instance" / "Permission denied"

**Cause**: Service account lacks permissions

**Solution**: Ensure service account has the custom role with required permissions (see GCP Configuration above)

## How It Works

```
Player connects to proxy
        ↓
Is player whitelisted? (if enabled)
   ↙           ↘
 NO             YES
  ↓              ↓
Kick player    Is backend server reachable?
             ↙           ↘
           YES            NO
            ↓              ↓
          Allow      Start GCP instance
       connection    ↓
                Kick player with message
                    ↓
                Player retries after 30-60s
                    ↓
                Connects successfully
                    ↓
                Player disconnects
                    ↓
                No players left?
                    ↓
                Start 30-min timer
                    ↓
                Stop GCP instance
```

## License

Distributed under the same license as Minekube Gate. See `LICENSE` for more information.

## Acknowledgments

- [Minekube Gate](https://github.com/minekube/gate) - The awesome Minecraft proxy
- [Google Cloud Compute Engine](https://cloud.google.com/compute) - Infrastructure provider

<p align="right">(<a href="#readme-top">back to top</a>)</p>
