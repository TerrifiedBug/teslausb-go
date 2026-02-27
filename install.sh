#!/bin/bash
set -euo pipefail

REPO="TerrifiedBug/teslausb-go"
MIKE_REPO="MikeBishop/tesla-vehicle-command-arm-binaries"

echo "=== teslausb-go installer ==="

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  aarch64) GOARCH="arm64"; TCARCH="armv7" ;;
  armv7l)  GOARCH="arm";   TCARCH="armv7" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Check for existing install
UPGRADE=false
if [ -f /usr/local/bin/teslausb ]; then
  echo "Existing install detected — upgrading binary only"
  UPGRADE=true
fi

# Download latest release tag
echo "Architecture: $ARCH (binary: $GOARCH, tesla-control: $TCARCH)"
LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d'"' -f4)
if [ -z "$LATEST" ]; then
  echo "ERROR: Could not determine latest release"
  exit 1
fi
echo "Latest release: $LATEST"

# Download teslausb binary
echo "Downloading teslausb..."
curl -fsSL "https://github.com/$REPO/releases/download/$LATEST/teslausb-linux-$GOARCH" -o /usr/local/bin/teslausb
chmod +x /usr/local/bin/teslausb
echo "Installed teslausb $(/usr/local/bin/teslausb -version 2>/dev/null || echo "$LATEST")"

# Download tesla-control (MikeBishop only publishes armv7 — runs fine on arm64)
if [ -f /usr/local/bin/tesla-control ]; then
  echo "tesla-control already installed, skipping"
else
  echo "Downloading tesla-control..."
  TC_TMP=$(mktemp -d)
  curl -fsSL "https://github.com/$MIKE_REPO/releases/latest/download/vehicle-command-binaries-linux-$TCARCH.tar.gz" \
    -o "$TC_TMP/tc.tar.gz"
  tar xzf "$TC_TMP/tc.tar.gz" -C "$TC_TMP"
  cp "$TC_TMP/tesla-control" /usr/local/bin/tesla-control
  chmod +x /usr/local/bin/tesla-control
  rm -rf "$TC_TMP"
  echo "Installed tesla-control"
fi

if [ "$UPGRADE" = true ]; then
  systemctl restart teslausb || true
  echo "Upgrade complete!"
  exit 0
fi

# First install — configure system
echo "Installing packages..."
apt-get update -qq >/dev/null
apt-get install -y -qq exfatprogs nfs-common rsync bluez fdisk ntpsec-ntpdate >/dev/null 2>&1

echo "Disabling unnecessary services..."
systemctl disable --now apt-daily.timer apt-daily-upgrade.timer dpkg-db-backup.timer 2>/dev/null || true
systemctl disable --now triggerhappy keyboard-setup 2>/dev/null || true
apt-get remove -y -qq dphys-swapfile >/dev/null 2>&1 || true

# Configure USB gadget boot
echo "Configuring USB gadget..."
if ! grep -q "dtoverlay=dwc2" /boot/firmware/config.txt; then
  echo "dtoverlay=dwc2" >> /boot/firmware/config.txt
fi
if ! grep -q "modules-load=dwc2,g_ether" /boot/firmware/cmdline.txt; then
  sed -i 's/$/ modules-load=dwc2,g_ether/' /boot/firmware/cmdline.txt
fi

# Create directories
mkdir -p /backingfiles /mnt/cam /mnt/archive /mutable/teslausb /mutable/ble /mutable/logs

# Default config
if [ ! -f /mutable/teslausb/config.yaml ]; then
  cat > /mutable/teslausb/config.yaml << 'YAML'
nfs:
  server: ""
  share: ""
keep_awake:
  method: "ble"
  vin: ""
  webhook_url: ""
notifications:
  webhook_url: ""
temperature:
  warning_celsius: 70
  caution_celsius: 60
YAML
fi

# Systemd service
cat > /etc/systemd/system/teslausb.service << 'SERVICE'
[Unit]
Description=teslausb-go
After=network-online.target bluetooth.target
Wants=network-online.target

[Service]
ExecStart=/usr/local/bin/teslausb
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
SERVICE

systemctl daemon-reload
systemctl enable -q teslausb

echo ""
echo "=== Setup complete! ==="
echo "Run 'sudo reboot' to enable USB gadget mode."
echo "After reboot, open http://$(hostname).local to configure."
