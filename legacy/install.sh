#!/usr/bin/env bash
# DEPRECATED: This script is superseded by the juniper-host Go binary.
# See README.md for current installation instructions.
# Kept for reference only.
#
# Juniper Bible - NixOS Host Installation Script
# Run this from the NixOS installer after partitioning and mounting

set -euo pipefail

REPO_BASE="https://raw.githubusercontent.com/JuniperBible/juniper-server/main"

echo "========================================"
echo "Juniper Bible - NixOS Host Installation"
echo "========================================"
echo ""

# Check if running as root
if [[ $EUID -ne 0 ]]; then
  echo "This script must be run as root (use sudo)"
  exit 1
fi

# Check if /mnt is mounted
if ! mountpoint -q /mnt; then
  echo "ERROR: /mnt is not mounted."
  echo ""
  echo "Please partition and mount your disk first:"
  echo ""
  echo "  # For /dev/sda (or /dev/vda on cloud VPS):"
  echo "  parted /dev/sda -- mklabel gpt"
  echo "  parted /dev/sda -- mkpart ESP fat32 1MB 512MB"
  echo "  parted /dev/sda -- set 1 esp on"
  echo "  parted /dev/sda -- mkpart primary 512MB 100%"
  echo "  mkfs.fat -F 32 -n boot /dev/sda1"
  echo "  mkfs.ext4 -L nixos /dev/sda2"
  echo "  mount /dev/sda2 /mnt"
  echo "  mkdir -p /mnt/boot"
  echo "  mount /dev/sda1 /mnt/boot"
  echo ""
  exit 1
fi

# Check if /mnt/boot is mounted
if ! mountpoint -q /mnt/boot; then
  echo "ERROR: /mnt/boot is not mounted."
  echo "Please mount your boot partition: mount /dev/sda1 /mnt/boot"
  exit 1
fi

echo "Step 1: Generating hardware configuration..."
nixos-generate-config --root /mnt

echo ""
echo "Step 2: Downloading Juniper Bible configuration..."
curl -fsSL "$REPO_BASE/configuration.nix" -o /mnt/etc/nixos/configuration.nix

echo ""
echo "Step 3: Installing NixOS..."
echo "(This will take a few minutes)"
echo ""
nixos-install --no-root-passwd

echo ""
echo "========================================"
echo "Installation complete!"
echo "========================================"
echo ""
echo "IMPORTANT: Before rebooting, you should:"
echo ""
echo "1. Edit /mnt/etc/nixos/configuration.nix to add your SSH key:"
echo "   nano /mnt/etc/nixos/configuration.nix"
echo ""
echo "   Find this line and add your key:"
echo '   users.users.deploy.openssh.authorizedKeys.keys = ['
echo '     "ssh-ed25519 AAAA... your-key-here"'
echo '   ];'
echo ""
echo "2. Set your domain (if not juniperbible.org):"
echo '   services.caddy.virtualHosts."your-domain.com".extraConfig = ...'
echo ""
echo "3. Rebuild to apply changes:"
echo "   nixos-install --no-root-passwd"
echo ""
echo "4. Reboot:"
echo "   reboot"
echo ""
echo "After reboot, SSH in as 'deploy' user and run:"
echo "   deploy-juniper"
echo ""
