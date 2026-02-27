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
  *)       echo "ERROR: Unsupported architecture: $ARCH"; exit 1 ;;
esac
echo "Architecture: $ARCH (Go: $GOARCH, tesla-control: $TCARCH)"

# Check for existing install
UPGRADE=false
if [ -f /usr/local/bin/teslausb ]; then
  echo "Existing install detected — upgrading binary only"
  UPGRADE=true
fi

# Download latest release tag
echo "Fetching latest release..."
RELEASE_URL="https://api.github.com/repos/$REPO/releases/latest"
LATEST=$(curl -fsSL "$RELEASE_URL" | grep tag_name | cut -d'"' -f4)
if [ -z "$LATEST" ]; then
  echo "ERROR: Could not determine latest release from $RELEASE_URL"
  exit 1
fi
echo "Latest release: $LATEST"

# Download teslausb binary
BINARY_URL="https://github.com/$REPO/releases/download/$LATEST/teslausb-linux-$GOARCH"
echo "Downloading teslausb from $BINARY_URL..."
curl -fsSL "$BINARY_URL" -o /usr/local/bin/teslausb
chmod +x /usr/local/bin/teslausb
echo "teslausb binary installed ($(/usr/local/bin/teslausb -version 2>/dev/null || echo 'unknown version'))"

# Download tesla-control (MikeBishop only publishes armv7 — runs fine on arm64)
TC_URL="https://github.com/$MIKE_REPO/releases/latest/download/vehicle-command-binaries-linux-$TCARCH.tar.gz"
echo "Downloading tesla-control from $TC_URL..."
TC_TMP=$(mktemp -d)
curl -fsSL "$TC_URL" -o "$TC_TMP/tc.tar.gz"
echo "Downloaded $(wc -c < "$TC_TMP/tc.tar.gz") bytes"
tar xzf "$TC_TMP/tc.tar.gz" -C "$TC_TMP"
ls -la "$TC_TMP/"
cp "$TC_TMP/tesla-control" /usr/local/bin/tesla-control
chmod +x /usr/local/bin/tesla-control
rm -rf "$TC_TMP"
echo "tesla-control installed"

if [ "$UPGRADE" = true ]; then
  echo "Restarting service..."
  systemctl restart teslausb || true
  echo "Upgrade complete!"
  exit 0
fi

# First install — configure system
echo "Installing packages..."
apt-get update -qq
apt-get install -y -qq exfatprogs nfs-common rsync bluez fdisk sntp

echo "Disabling unnecessary services..."
systemctl disable --now apt-daily.timer apt-daily-upgrade.timer dpkg-db-backup.timer 2>/dev/null || true
systemctl disable --now triggerhappy keyboard-setup 2>/dev/null || true
apt-get remove -y -qq dphys-swapfile 2>/dev/null || true

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
systemctl enable teslausb

HOSTNAME=$(hostname)
echo ""
echo "=== Setup complete! ==="
echo "A reboot is required to enable USB gadget mode."
echo "After reboot, open http://$HOSTNAME.local to configure."
echo ""
echo "Reboot now? (y/N)"
read -r REPLY
if [ "$REPLY" = "y" ] || [ "$REPLY" = "Y" ]; then
  reboot
fi
