#!/usr/bin/env bash
# Juniper Bible - First Login Setup Wizard
# Runs automatically on first login, then disables itself

set -euo pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m' # No Color

# Config file
NIXOS_CONFIG="/etc/nixos/configuration.nix"
SETUP_DONE_FLAG="/etc/juniper-setup-complete"

# Check if already completed
if [[ -f "$SETUP_DONE_FLAG" ]]; then
  exit 0
fi

# Gather system info
SYS_HOSTNAME=$(hostname)
SYS_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "N/A")
SYS_OS=$(grep VERSION_ID /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"' || echo "NixOS")
SYS_KERNEL=$(uname -r)

clear
echo -e "${CYAN}"
echo "                                 ▄"
echo "                                ▟ ▙"
echo "                               ▟   ▙"
echo "                              ▟     ▙"
echo "                             ▟       ▙"
echo "                            ▟         ▙"
echo "                           ▟   ▄███▄   ▙"
echo "                          ▟  ▄█▀   ▀█▄  ▙"
echo "                         ▟  ██       ██  ▙"
echo "                        ▟   █    ●    █   ▙"
echo "                       ▟    ██       ██    ▙"
echo "                      ▟      ▀█▄   ▄█▀      ▙"
echo "                     ▟         ▀███▀         ▙"
echo "                    ▟                         ▙"
echo "                   ▟                           ▙"
echo "                  ▟▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▄▙"
echo ""
echo "               ╦╦ ╦╔╗╔╦╔═╗╔═╗╦═╗  ╔╗ ╦╔╗ ╦  ╔═╗"
echo "               ║║ ║║║║║╠═╝║╣ ╠╦╝  ╠╩╗║╠╩╗║  ║╣"
echo "              ╚╝╚═╝╝╚╝╩╩  ╚═╝╩╚═  ╚═╝╩╚═╝╩═╝╚═╝"
echo -e "${NC}"
echo -e "${BOLD}                Welcome to Juniper Bible Server${NC}"
echo "                ─────────────────────────────────"
echo "                Hostname:  $SYS_HOSTNAME"
echo "                IP:        $SYS_IP"
echo "                OS:        NixOS $SYS_OS"
echo "                Kernel:    $SYS_KERNEL"
echo ""
echo -e "${YELLOW}                Press Enter to continue...${NC}"
read -r

# =============================================================================
# Step 1: Hostname
# =============================================================================
clear
echo -e "${BOLD}Step 1/4: Hostname${NC}"
echo ""
current_hostname=$(hostname)
echo -e "Current hostname: ${CYAN}$current_hostname${NC}"
echo ""
read -p "Enter new hostname (or press Enter to keep current): " new_hostname
new_hostname="${new_hostname:-$current_hostname}"

# =============================================================================
# Step 2: Domain
# =============================================================================
clear
echo -e "${BOLD}Step 2/4: Domain${NC}"
echo ""
echo "Enter your domain for HTTPS (Caddy will auto-provision certificates)."
echo ""
echo "Examples:"
echo "  - juniperbible.org"
echo "  - bible.example.com"
echo "  - localhost (for testing, no HTTPS)"
echo ""
read -p "Domain: " domain
domain="${domain:-localhost}"

# =============================================================================
# Step 3: SSH Keys
# =============================================================================
clear
echo -e "${BOLD}Step 3/4: SSH Keys${NC}"
echo ""
echo "Add SSH public keys for the 'deploy' user."
echo "Paste one key per line. Enter empty line when done."
echo ""
echo -e "${YELLOW}WARNING: If you don't add a key, you may be locked out!${NC}"
echo ""

ssh_keys=()
while true; do
  read -p "SSH key (or Enter to finish): " key
  if [[ -z "$key" ]]; then
    break
  fi
  if [[ "$key" == ssh-* ]] || [[ "$key" == ecdsa-* ]]; then
    ssh_keys+=("$key")
    echo -e "${GREEN}✓ Key added${NC}"
  else
    echo -e "${RED}Invalid key format. Keys should start with ssh-ed25519, ssh-rsa, or ecdsa-*${NC}"
  fi
done

if [[ ${#ssh_keys[@]} -eq 0 ]]; then
  echo ""
  echo -e "${RED}No SSH keys added! You may be locked out after reboot.${NC}"
  read -p "Continue anyway? [y/N] " -n 1 -r
  echo
  if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Setup cancelled. Run 'setup-wizard' to try again."
    exit 1
  fi
fi

# =============================================================================
# Step 4: Deploy Site Now?
# =============================================================================
clear
echo -e "${BOLD}Step 4/4: Deploy Site${NC}"
echo ""
echo "Would you like to deploy Juniper Bible now?"
echo ""
read -p "Deploy site? [Y/n] " -n 1 -r deploy_now
echo
deploy_now="${deploy_now:-y}"

# =============================================================================
# Summary
# =============================================================================
clear
echo -e "${BOLD}Configuration Summary${NC}"
echo ""
echo -e "  Hostname: ${CYAN}$new_hostname${NC}"
echo -e "  Domain:   ${CYAN}$domain${NC}"
echo -e "  SSH Keys: ${CYAN}${#ssh_keys[@]} key(s)${NC}"
echo -e "  Deploy:   ${CYAN}$([ "$deploy_now" =~ ^[Yy]$ ] && echo "Yes" || echo "No")${NC}"
echo ""
read -p "Apply this configuration? [Y/n] " -n 1 -r confirm
echo
confirm="${confirm:-y}"

if [[ ! $confirm =~ ^[Yy]$ ]]; then
  echo "Setup cancelled. Run 'setup-wizard' to try again."
  exit 1
fi

# =============================================================================
# Apply Configuration
# =============================================================================
echo ""
echo -e "${BOLD}Applying configuration...${NC}"

# Build SSH keys string
ssh_keys_nix=""
for key in "${ssh_keys[@]}"; do
  ssh_keys_nix+="    \"$key\"\n"
done

# Update configuration.nix
sudo cp "$NIXOS_CONFIG" "${NIXOS_CONFIG}.backup"

# Update hostname
sudo sed -i "s/networking.hostName = \".*\"/networking.hostName = \"$new_hostname\"/" "$NIXOS_CONFIG"

# Update domain
sudo sed -i "s/services.caddy.virtualHosts.\".*\".extraConfig/services.caddy.virtualHosts.\"$domain\".extraConfig/" "$NIXOS_CONFIG"

# Update SSH keys
if [[ ${#ssh_keys[@]} -gt 0 ]]; then
  # Create temp file with new keys
  keys_block="users.users.deploy.openssh.authorizedKeys.keys = [\n${ssh_keys_nix}  ];"
  sudo sed -i '/users.users.deploy.openssh.authorizedKeys.keys/,/];/c\'"$keys_block" "$NIXOS_CONFIG" 2>/dev/null || true
fi

echo -e "${GREEN}✓ Configuration updated${NC}"

# Rebuild NixOS
echo ""
echo "Rebuilding NixOS (this may take a minute)..."
if sudo nixos-rebuild switch; then
  echo -e "${GREEN}✓ NixOS rebuilt successfully${NC}"
else
  echo -e "${RED}✗ NixOS rebuild failed. Check /etc/nixos/configuration.nix${NC}"
  echo "  Backup saved to ${NIXOS_CONFIG}.backup"
  exit 1
fi

# Mark setup as complete
sudo touch "$SETUP_DONE_FLAG"

# Deploy site if requested
if [[ $deploy_now =~ ^[Yy]$ ]]; then
  echo ""
  echo "Deploying Juniper Bible..."
  if sudo /etc/deploy-juniper.sh; then
    echo -e "${GREEN}✓ Site deployed successfully${NC}"
  else
    echo -e "${YELLOW}Site deployment failed. You can try again with: deploy-juniper${NC}"
  fi
fi

# =============================================================================
# Done
# =============================================================================
echo ""
echo -e "${GREEN}${BOLD}Setup Complete!${NC}"
echo ""
echo "Your Juniper Bible server is ready."
echo ""
if [[ "$domain" != "localhost" ]]; then
  echo -e "  Website: ${CYAN}https://$domain${NC}"
else
  echo -e "  Website: ${CYAN}http://$domain${NC}"
fi
echo -e "  SSH:     ${CYAN}ssh deploy@$(hostname -I | awk '{print $1}')${NC}"
echo ""
echo "Useful commands:"
echo "  deploy-juniper     - Update the site"
echo "  sudo nixos-rebuild switch - Apply config changes"
echo ""
