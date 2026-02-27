#!/bin/bash
set -euo pipefail

REPO="TerrifiedBug/teslausb-go"
MIKE_REPO="MikeBishop/tesla-vehicle-command-arm-binaries"

echo "=== teslausb-go installer ==="

# Detect architecture
ARCH=$(uname -m)
case "$ARCH" in
  aarch64) GOARCH="arm64" ;;
  armv7l)  GOARCH="arm" ;;
  *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Check for existing install
UPGRADE=false
if [ -f /usr/local/bin/teslausb ]; then
  echo "Existing install detected — upgrading binary only"
  UPGRADE=true
fi

# Download latest binary
echo "Downloading teslausb binary..."
LATEST=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" | grep tag_name | cut -d'"' -f4)
curl -sSL "https://github.com/$REPO/releases/download/$LATEST/teslausb-linux-$GOARCH" -o /usr/local/bin/teslausb
chmod +x /usr/local/bin/teslausb

# Download tesla-control
echo "Downloading tesla-control..."
curl -sSL "https://github.com/$MIKE_REPO/releases/latest/download/vehicle-command-binaries-linux-$GOARCH.tar.gz" \
  | tar xzf - -C /usr/local/bin/ --strip-components=1

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
