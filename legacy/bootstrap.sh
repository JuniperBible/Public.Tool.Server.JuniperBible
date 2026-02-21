#!/usr/bin/env bash
# DEPRECATED: This script is superseded by the juniper-host Go binary.
# See README.md for current installation instructions.
# Kept for reference only.
#
# Juniper Bible - NixOS One-Liner Bootstrap
# Usage: curl -fsSL https://raw.githubusercontent.com/JuniperBible/juniper-server/main/bootstrap.sh | sudo bash -s -- [DISK] [SSH_KEY]
#
# Examples:
#   curl ... | sudo bash                                    # Auto-detects disk, prompts for SSH key
#   curl ... | sudo bash -s -- /dev/vda                     # Uses /dev/vda, prompts for SSH key
#   curl ... | sudo bash -s -- /dev/vda "ssh-ed25519 AAAA..." # Full automation

set -euo pipefail

# Auto-detect disk if not provided
detect_disk() {
  # Check for common disk types in order of preference
  for disk in /dev/vda /dev/sda /dev/nvme0n1 /dev/xvda; do
    if [[ -b "$disk" ]]; then
      echo "$disk"
      return
    fi
  done
  echo ""
}

DISK="${1:-$(detect_disk)}"
SSH_KEY="${2:-}"
REPO_BASE="https://raw.githubusercontent.com/JuniperBible/juniper-server/main"

if [[ -z "$DISK" ]]; then
  echo "ERROR: Could not detect disk. Please specify: curl ... | sudo bash -s -- /dev/sdX"
  exit 1
fi

echo "========================================"
echo "Juniper Bible - NixOS Bootstrap"
echo "========================================"
echo "Disk: $DISK"
echo ""

# Check root
[[ $EUID -eq 0 ]] || { echo "Run as root: curl ... | sudo bash"; exit 1; }

# Confirm disk
echo "WARNING: This will ERASE $DISK"
if [[ -z "$SSH_KEY" ]]; then
  read -p "Continue? [y/N] " -n 1 -r
  echo
  [[ $REPLY =~ ^[Yy]$ ]] || exit 1
fi

# Partition
echo "Partitioning $DISK..."
parted "$DISK" -- mklabel gpt
parted "$DISK" -- mkpart ESP fat32 1MB 512MB
parted "$DISK" -- set 1 esp on
parted "$DISK" -- mkpart primary 512MB 100%
sleep 2

# Determine partition suffix (nvme uses p1/p2, others use 1/2)
if [[ "$DISK" == *"nvme"* ]]; then
  PART1="${DISK}p1"
  PART2="${DISK}p2"
else
  PART1="${DISK}1"
  PART2="${DISK}2"
fi

# Format
echo "Formatting..."
mkfs.fat -F 32 -n boot "$PART1"
mkfs.ext4 -F -L nixos "$PART2"

# Wait for labels to be recognized
echo "Waiting for disk labels..."
udevadm settle
sleep 2

# Mount
echo "Mounting..."
mount "$PART2" /mnt
mkdir -p /mnt/boot
mount "$PART1" /mnt/boot

# Generate hardware config
echo "Generating hardware configuration..."
nixos-generate-config --root /mnt

# Download configuration
echo "Downloading configuration..."
curl -fsSL "$REPO_BASE/configuration.nix" -o /mnt/etc/nixos/configuration.nix

# Get SSH key if not provided
if [[ -z "$SSH_KEY" ]]; then
  echo ""
  echo "Enter your SSH public key (ssh-ed25519 or ssh-rsa):"
  read -r SSH_KEY
fi

# Inject SSH key
if [[ -n "$SSH_KEY" ]]; then
  sed -i "s|# \"ssh-ed25519 AAAA... your-key-here\"|\"$SSH_KEY\"|" /mnt/etc/nixos/configuration.nix
  echo "SSH key configured."
fi

# Install
echo "Installing NixOS (this takes a few minutes)..."
nixos-install --no-root-passwd

echo ""
echo "========================================"
echo "Installation complete!"
echo "========================================"
echo ""
echo "Rebooting in 5 seconds... (Ctrl+C to cancel)"
sleep 5
reboot
