# teslausb-go

A ground-up rewrite of [TeslaUSB](https://github.com/marcone/teslausb) as a single Go binary with an embedded React web UI, targeting the Raspberry Pi Zero 2 W.

The original project spans roughly 14,000 lines of bash, Python, and JavaScript. This rewrite replaces it with approximately 3,500 lines of Go and TypeScript. The scope is deliberately narrower: dashcam/sentry footage only (no music or lightshow), NFS-only archiving, and a single webhook notification channel.

## Features

- **USB mass storage gadget** -- presents a virtual USB drive to the Tesla for dashcam and sentry recording
- **NFS archiving with rsync** -- mounts a remote NFS share and syncs footage when the car is parked on WiFi
- **BLE or webhook keep-awake** -- prevents the car from sleeping during archive via Bluetooth LE or an HTTP webhook
- **Temperature monitoring** -- reads the Pi's CPU temperature with configurable warning and caution thresholds
- **Real-time web dashboard** -- embedded React UI showing system state, temperatures, and archive progress
- **Self-updating** -- update the binary via the command line without touching config or footage

## Quick Start

### Prerequisites

1. Flash **Raspberry Pi OS Lite (64-bit, Bookworm)** to an SD card using [Raspberry Pi Imager](https://www.raspberrypi.com/software/).
2. In the imager, configure WiFi credentials and enable SSH before writing.
3. Insert the SD card into a Raspberry Pi Zero 2 W and boot it.

### Install

SSH into the Pi and run:

```bash
curl -sSL https://raw.githubusercontent.com/TerrifiedBug/teslausb-go/main/install.sh | sudo bash
```

The installer downloads the binary and `tesla-control`, installs required system packages (`exfatprogs`, `nfs-common`, `rsync`, `bluez`, etc.), configures the USB gadget overlay, creates a systemd service, and prompts for a reboot.

### Tesla Vehicle Settings

Recent Tesla firmware cuts power to USB ports when the car sleeps. To keep the car awake during archiving:

1. **Safety → Sentry Mode**: Set to **ON**
2. **Exclude Home**: **Unchecked**

This ensures Sentry Mode is active when you arrive home, keeping the car (and USB power) awake while teslausb-go archives footage. After archiving completes, teslausb-go turns Sentry Mode off so the car can sleep and conserve battery.

**Option A: BLE (Bluetooth Low Energy)**

Pair the Pi directly with your car via Bluetooth. No external services needed.

1. In the teslausb-go web UI: enter your VIN and click **Pair**, then tap your NFC key card on the center console
2. After archiving, teslausb-go sends `sentry-mode off` via BLE

**Option B: Webhook**

Use an external service like Home Assistant to turn Sentry Mode off after archiving.

1. In the teslausb-go web UI: set keep-awake method to **Webhook** and enter your webhook URL
2. After archiving, teslausb-go sends a webhook with `{"awake_command":"stop"}`
3. Your automation turns off Sentry Mode via the Tesla integration

Example Home Assistant automation:

```yaml
triggers:
  - trigger: webhook
    webhook_id: teslausb-keep-awake
    allowed_methods: [POST]
    local_only: false
actions:
  - choose:
      - conditions:
          - condition: template
            value_template: "{{ trigger.json.awake_command == 'stop' }}"
        sequence:
          - action: switch.turn_off
            target:
              entity_id: switch.your_car_sentry_mode
```

### Configure

After reboot, open `http://teslausb.local` in a browser and fill in your NFS server, share path, and keep-awake settings.

## Configuration

All configuration is managed through the web UI and stored at `/mutable/teslausb/config.yaml`. The file can also be edited directly. The available keys are:

```yaml
nfs:
  server: "192.168.1.100"       # NFS server IP or hostname
  share: "/volume1/TeslaCam"    # NFS export path

keep_awake:
  method: "ble"                 # "ble" or "webhook"
  vin: ""                       # Vehicle VIN (required for BLE)
  webhook_url: ""               # Webhook URL (required for webhook method)

notifications:
  webhook_url: ""               # URL for archive/error notifications

temperature:
  warning_celsius: 70           # Threshold for warning state
  caution_celsius: 60           # Threshold for caution state
```

WiFi is configured via Raspberry Pi Imager, not in this file.

## Development

### Requirements

- Go 1.22+
- Node.js 20+

### Commands

```bash
# Run locally with example config
make dev

# Start the React dev server with hot reload
make dev-web

# Build the web UI and embed it, then build the host binary
make build-web && make build

# Cross-compile for Raspberry Pi Zero 2 W (linux/arm64)
make build-arm64

# Run tests
make test

# Remove build artifacts
make clean
```

The `make dev` target builds the Go binary and runs it with `config.yaml.example`. The web dev server (`make dev-web`) runs separately via Vite for frontend development.

## Architecture

### State Machine

The system operates as a state machine with five states:

```
away --> arriving --> archiving --> idle --> away
```

- **away** -- the car is disconnected from WiFi (driving or parked elsewhere)
- **arriving** -- WiFi connection detected; the system prepares to archive
- **archiving** -- footage is being synced to the NFS share via rsync; keep-awake is active
- **idle** -- archiving is complete; the system waits quietly
- **away** -- WiFi drops; the USB gadget is re-presented for recording

### Internal Packages

The `internal/` directory contains the following packages:

| Package | Purpose |
|---------|---------|
| `archive` | NFS mount and rsync-based footage sync |
| `ble` | Bluetooth LE keep-awake via tesla-control |
| `config` | YAML config loading and validation |
| `disk` | Backing file and partition management |
| `gadget` | USB mass storage gadget setup |
| `monitor` | CPU temperature monitoring |
| `notify` | Webhook notifications |
| `state` | State machine and transitions |
| `system` | Hostname, reboot, and system-level operations |
| `update` | Binary self-update from GitHub releases |
| `web` | HTTP server and embedded React static files |
| `webhook` | Webhook-based keep-awake |

### Embedded Web UI

The React frontend (in `web/`) is built with Vite and TypeScript. At build time, the compiled assets are copied into `internal/web/static/` and embedded into the Go binary using `embed.FS`, so there are no external files to deploy.

## Updating

Update the binary via the web UI or from the command line:

```bash
sudo teslausb update
```

Updates download the latest release from GitHub and replace the binary in place. Configuration (`/mutable/teslausb/config.yaml`) and recorded footage are never modified.

## License

MIT
