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

# Check if already on latest version
CURRENT_VERSION=$(/usr/local/bin/teslausb -version 2>/dev/null || echo "none")
if [ "$CURRENT_VERSION" = "$LATEST" ]; then
  echo "Already on $LATEST, skipping download"
else
  echo "Downloading teslausb ($CURRENT_VERSION -> $LATEST)..."
  curl -fsSL "https://github.com/$REPO/releases/download/$LATEST/teslausb-linux-$GOARCH" -o /usr/local/bin/teslausb.new
  mv /usr/local/bin/teslausb.new /usr/local/bin/teslausb
  chmod +x /usr/local/bin/teslausb
  echo "Installed teslausb $(/usr/local/bin/teslausb -version 2>/dev/null || echo "$LATEST")"
fi

# Download tesla-control + tesla-keygen (MikeBishop only publishes armv7 — runs fine on arm64)
if [ -f /usr/local/bin/tesla-control ] && [ -f /usr/local/bin/tesla-keygen ]; then
  echo "tesla-control + tesla-keygen already installed, skipping"
else
  echo "Downloading tesla vehicle-command binaries..."
  TC_TMP=$(mktemp -d)
  curl -fsSL "https://github.com/$MIKE_REPO/releases/latest/download/vehicle-command-binaries-linux-$TCARCH.tar.gz" \
    -o "$TC_TMP/tc.tar.gz"
  tar xzf "$TC_TMP/tc.tar.gz" -C "$TC_TMP"
  cp "$TC_TMP/tesla-control" /usr/local/bin/tesla-control
  cp "$TC_TMP/tesla-keygen" /usr/local/bin/tesla-keygen
  chmod +x /usr/local/bin/tesla-control /usr/local/bin/tesla-keygen
  rm -rf "$TC_TMP"
  echo "Installed tesla-control + tesla-keygen"
fi

if [ "$UPGRADE" = true ]; then
  systemctl restart teslausb || true
  echo "Upgrade complete!"
  exit 0
fi

# First install — configure system
echo "Installing packages..."
apt-get update -qq >/dev/null
apt-get install -y -qq exfatprogs nfs-common cifs-utils rsync bluez fdisk ntpsec-ntpdate >/dev/null 2>&1

echo "Disabling unnecessary services..."
systemctl disable --now apt-daily.timer apt-daily-upgrade.timer dpkg-db-backup.timer 2>/dev/null || true
systemctl disable --now triggerhappy keyboard-setup 2>/dev/null || true
apt-get remove -y -qq dphys-swapfile >/dev/null 2>&1 || true

# Configure USB gadget boot
echo "Configuring USB gadget..."
# Remove any dwc2 overlay with host mode (breaks gadget mode)
sed -i '/dtoverlay=dwc2,dr_mode=host/d' /boot/firmware/config.txt
# Add dtoverlay=dwc2 under [all] section so it applies to all Pi models
# (config.txt has [cm4], [cm5] sections that only apply to Compute Modules)
if ! grep -q "^dtoverlay=dwc2$" /boot/firmware/config.txt; then
  if grep -q "^\[all\]" /boot/firmware/config.txt; then
    sed -i '/^\[all\]/a dtoverlay=dwc2' /boot/firmware/config.txt
  else
    echo -e "\n[all]\ndtoverlay=dwc2" >> /boot/firmware/config.txt
  fi
fi
if ! grep -q "modules-load=dwc2" /boot/firmware/cmdline.txt; then
  sed -i 's/$/ modules-load=dwc2,g_ether/' /boot/firmware/cmdline.txt
fi

# Create directories
mkdir -p /backingfiles /mnt/cam /mnt/archive /mutable/teslausb /mutable/ble /mutable/logs

# Default config
if [ ! -f /mutable/teslausb/config.yaml ]; then
  cat > /mutable/teslausb/config.yaml << 'YAML'
archive:
  recent_clips: false
  reserve_percent: 10
  method: "nfs"
nfs:
  server: ""
  share: ""
cifs:
  server: ""
  share: ""
  username: ""
  password: ""
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

# Read-only root filesystem (protect SD card from power-loss corruption)
echo "Configuring read-only root filesystem..."

# Add ro, fastboot, noswap to kernel cmdline if not present
if ! grep -q '\bro\b' /boot/firmware/cmdline.txt; then
  sed -i 's/$/ fastboot noswap ro/' /boot/firmware/cmdline.txt
fi

# Add tmpfs mounts for writable paths
if ! grep -q 'tmpfs.*/tmp' /etc/fstab; then
  cat >> /etc/fstab << 'FSTAB'

# tmpfs for read-only root
tmpfs /tmp tmpfs nosuid,nodev 0 0
tmpfs /var/log tmpfs nosuid,nodev,noexec,size=32M 0 0
tmpfs /var/tmp tmpfs nosuid,nodev 0 0
tmpfs /var/lib/systemd tmpfs nosuid,nodev 0 0
tmpfs /var/lib/dhcpcd tmpfs nosuid,nodev 0 0
FSTAB
fi

# Mark root as read-only in fstab
if grep -q '/ .*ext4' /etc/fstab; then
  sed -i 's|\(.*/ .*ext4.*\)defaults\(.*\)|\1defaults,ro\2|' /etc/fstab
fi

echo ""
echo "=== Setup complete! ==="
echo "Root filesystem is read-only. /mutable/ remains writable for config and data."
echo "Run 'sudo reboot' to enable USB gadget mode."
echo "After reboot, open http://$(hostname).local to configure."
