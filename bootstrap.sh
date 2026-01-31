#!/usr/bin/env bash
# Juniper Bible - NixOS One-Liner Bootstrap
# Usage: curl -fsSL https://raw.githubusercontent.com/FocuswithJustin/Website.Public.JuniperBible.org/main/nix-host/bootstrap.sh | sudo bash -s -- [DISK] [SSH_KEY]
#
# Examples:
#   curl ... | sudo bash                                    # Uses /dev/sda, prompts for SSH key
#   curl ... | sudo bash -s -- /dev/vda                     # Uses /dev/vda, prompts for SSH key
#   curl ... | sudo bash -s -- /dev/sda "ssh-ed25519 AAAA..." # Full automation

set -euo pipefail

DISK="${1:-/dev/sda}"
SSH_KEY="${2:-}"
REPO_BASE="https://raw.githubusercontent.com/JuniperBible/Website.Server.JuniperBible.org/main"

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

# Format
echo "Formatting..."
mkfs.fat -F 32 -n boot "${DISK}1"
mkfs.ext4 -F -L nixos "${DISK}2"

# Mount
echo "Mounting..."
mount /dev/disk/by-label/nixos /mnt
mkdir -p /mnt/boot
mount /dev/disk/by-label/boot /mnt/boot

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
