{ config, pkgs, ... }:

{
  imports = [
    ./hardware-configuration.nix
  ];

  # ==========================================================================
  # CUSTOMIZE THESE VALUES
  # ==========================================================================

  networking.hostName = "juniperbible";

  # Your domain (used by Caddy for automatic HTTPS)
  # Set to your actual domain or use localhost for testing
  services.caddy.virtualHosts."juniperbible.org".extraConfig = ''
    root * /var/www/juniperbible
    encode gzip
    file_server {
      precompressed br gzip
    }

    # Cache static assets
    @static {
      path *.css *.js *.woff2 *.png *.jpg *.svg *.ico
    }
    header @static Cache-Control "public, max-age=31536000, immutable"

    # Cache Bible pages
    @bible {
      path /bible/*
    }
    header @bible Cache-Control "public, max-age=86400"

    # Security headers
    header {
      Strict-Transport-Security "max-age=31536000; includeSubDomains"
      X-Content-Type-Options nosniff
      X-Frame-Options DENY
      Referrer-Policy strict-origin-when-cross-origin
      Permissions-Policy "camera=(), microphone=(), geolocation=()"
    }
  '';

  # Add your SSH public key here
  users.users.deploy.openssh.authorizedKeys.keys = [
    # "ssh-ed25519 AAAA... your-key-here"
  ];

  # ==========================================================================
  # SYSTEM CONFIGURATION (usually no changes needed)
  # ==========================================================================

  # Boot loader - GRUB with hybrid BIOS+UEFI support
  # Works with GPT disks that have both bios_grub and ESP partitions
  boot.loader.grub = {
    enable = true;
    device = "/dev/vda";  # REPLACE_BOOT_DEVICE - installer will update this
    efiSupport = true;
    efiInstallAsRemovable = true;  # Install to ESP without NVRAM
  };
  boot.loader.efi.canTouchEfiVariables = false;

  # Networking
  networking.firewall.allowedTCPPorts = [ 22 80 443 ];

  # Security hardening
  boot.kernel.sysctl = {
    # Network hardening
    "net.ipv4.tcp_syncookies" = 1;
    "net.ipv4.conf.all.send_redirects" = 0;
    "net.ipv4.conf.default.send_redirects" = 0;
    "net.ipv4.conf.all.accept_redirects" = 0;
    "net.ipv4.conf.default.accept_redirects" = 0;
    "net.ipv4.conf.all.rp_filter" = 1;
    "net.ipv4.conf.default.rp_filter" = 1;
    "net.ipv4.conf.all.accept_source_route" = 0;
    "net.ipv4.conf.default.accept_source_route" = 0;
    "net.ipv4.icmp_echo_ignore_broadcasts" = 1;
    "net.ipv4.icmp_ignore_bogus_error_responses" = 1;
    # Kernel hardening
    "kernel.yama.ptrace_scope" = 1;
    "kernel.kptr_restrict" = 2;
    "kernel.dmesg_restrict" = 1;
    "kernel.perf_event_paranoid" = 3;
  };

  # Timezone
  time.timeZone = "UTC";

  # System packages
  environment.systemPackages = with pkgs; [
    vim
    git
    curl
    wget
    htop
    xz
    brotli
  ];

  # Caddy web server
  services.caddy.enable = true;

  # Deploy user (for SSH deployments)
  users.users.deploy = {
    isNormalUser = true;
    home = "/home/deploy";
    extraGroups = [ "caddy" ];
    shell = pkgs.bash;
  };

  # Web directory
  systemd.tmpfiles.rules = [
    "d /var/www/juniperbible 0755 deploy caddy -"
  ];

  # Deployment script
  environment.etc."deploy-juniper.sh" = {
    mode = "0755";
    text = ''
      #!/usr/bin/env bash
      set -euo pipefail

      RELEASE_URL="https://github.com/JuniperBible/Website.Server.JuniperBible.org/releases/latest/download/site.tar.xz"
      DEPLOY_DIR="/var/www/juniperbible"
      TEMP_DIR=$(mktemp -d)

      echo "Downloading latest release..."
      if ! curl -fsSL "$RELEASE_URL" -o "$TEMP_DIR/site.tar.xz"; then
        echo "ERROR: Failed to download release from $RELEASE_URL"
        echo "Please check that a release exists with site.tar.xz attached."
        rm -rf "$TEMP_DIR"
        exit 1
      fi

      echo "Extracting to $DEPLOY_DIR..."
      rm -rf "$DEPLOY_DIR"/*
      mkdir -p "$DEPLOY_DIR"
      tar -xJf "$TEMP_DIR/site.tar.xz" -C "$DEPLOY_DIR"

      echo "Cleaning up..."
      rm -rf "$TEMP_DIR"

      echo "Done! Site deployed to $DEPLOY_DIR"
    '';
  };

  # Setup wizard script (runs on first login)
  environment.etc."setup-wizard.sh" = {
    mode = "0755";
    text = ''
      #!/usr/bin/env bash
      # Juniper Bible - First Login Setup Wizard

      set -euo pipefail

      NIXOS_CONFIG="/etc/nixos/configuration.nix"
      SETUP_DONE_FLAG="/etc/juniper-setup-complete"

      [[ -f "$SETUP_DONE_FLAG" ]] && exit 0

      # Gather system info
      SYS_HOSTNAME=$(hostname)
      SYS_IP=$(hostname -I 2>/dev/null | awk '{print $1}' || echo "N/A")
      SYS_OS=$(grep VERSION_ID /etc/os-release 2>/dev/null | cut -d= -f2 | tr -d '"' || echo "NixOS")
      SYS_KERNEL=$(uname -r)

      clear
      echo ""
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
      echo ""
      echo "                Welcome to Juniper Bible Server"
      echo "                ─────────────────────────────────"
      echo "                Hostname:  $SYS_HOSTNAME"
      echo "                IP:        $SYS_IP"
      echo "                OS:        NixOS $SYS_OS"
      echo "                Kernel:    $SYS_KERNEL"
      echo ""
      echo "                Press Enter to continue..."
      read -r

      # Step 1: Hostname
      clear
      echo "Step 1/4: Hostname"
      echo ""
      current_hostname=$(hostname)
      echo "Current: $current_hostname"
      read -p "New hostname (Enter to keep): " new_hostname
      new_hostname="''${new_hostname:-$current_hostname}"

      # Step 2: Domain
      clear
      echo "Step 2/4: Domain"
      echo ""
      echo "Enter domain for HTTPS (e.g., juniperbible.org)"
      echo "Use 'localhost' for testing without HTTPS"
      echo ""
      read -p "Domain: " domain
      domain="''${domain:-localhost}"

      # Step 3: SSH Keys
      clear
      echo "Step 3/4: SSH Keys"
      echo ""
      echo "Paste SSH public keys (one per line, empty line to finish)"
      echo ""
      ssh_keys=()
      while true; do
        read -p "Key: " key
        [[ -z "$key" ]] && break
        if [[ "$key" == ssh-* ]] || [[ "$key" == ecdsa-* ]]; then
          ssh_keys+=("$key")
          echo "✓ Added"
        else
          echo "Invalid format (should start with ssh-ed25519, ssh-rsa, etc)"
        fi
      done

      # Step 4: Deploy now?
      clear
      echo "Step 4/4: Deploy Site"
      echo ""
      read -p "Deploy Juniper Bible now? [Y/n] " -n 1 -r deploy_now
      echo
      deploy_now="''${deploy_now:-y}"

      # Confirm
      clear
      echo "Configuration Summary"
      echo ""
      echo "  Hostname: $new_hostname"
      echo "  Domain:   $domain"
      echo "  SSH Keys: ''${#ssh_keys[@]}"
      echo ""
      read -p "Apply? [Y/n] " -n 1 -r confirm
      echo
      [[ ! $confirm =~ ^[Yy]?$ ]] && { echo "Cancelled."; exit 1; }

      # Apply
      echo "Applying configuration..."
      sudo cp "$NIXOS_CONFIG" "''${NIXOS_CONFIG}.backup"
      sudo sed -i "s/networking.hostName = \".*\"/networking.hostName = \"$new_hostname\"/" "$NIXOS_CONFIG"
      sudo sed -i "s/services.caddy.virtualHosts.\"[^\"]*\".extraConfig/services.caddy.virtualHosts.\"$domain\".extraConfig/" "$NIXOS_CONFIG"

      if [[ ''${#ssh_keys[@]} -gt 0 ]]; then
        keys_nix=""
        for key in "''${ssh_keys[@]}"; do
          keys_nix+="    \"$key\"\n"
        done
        sudo sed -i '/authorizedKeys.keys = \[/,/\];/{/authorizedKeys.keys = \[/!{/\];/!d}}' "$NIXOS_CONFIG"
        sudo sed -i "s|authorizedKeys.keys = \[|authorizedKeys.keys = [\n$keys_nix|" "$NIXOS_CONFIG"
      fi

      echo "Rebuilding NixOS..."
      if sudo nixos-rebuild switch; then
        echo "✓ Done"
      else
        echo "✗ Failed - check $NIXOS_CONFIG"
        exit 1
      fi

      sudo touch "$SETUP_DONE_FLAG"

      if [[ $deploy_now =~ ^[Yy]?$ ]]; then
        echo "Deploying site..."
        sudo /etc/deploy-juniper.sh || echo "Deploy failed - run 'deploy-juniper' to retry"
      fi

      echo ""
      echo "Setup Complete!"
      echo ""
      [[ "$domain" != "localhost" ]] && echo "  https://$domain" || echo "  http://localhost"
      echo "  ssh deploy@$(hostname -I | awk '{print $1}')"
      echo ""
    '';
  };

  # Add scripts to PATH
  environment.shellAliases = {
    deploy-juniper = "sudo /etc/deploy-juniper.sh";
    setup-wizard = "/etc/setup-wizard.sh";
  };

  # Run setup wizard on first login for deploy user
  programs.bash.interactiveShellInit = ''
    if [[ ! -f /etc/juniper-setup-complete ]] && [[ $USER == "deploy" ]]; then
      /etc/setup-wizard.sh
    fi
  '';

  # Fail2ban for SSH brute-force protection
  services.fail2ban = {
    enable = true;
    maxretry = 5;
    bantime = "1h";
    jails.sshd = {
      settings = {
        enabled = true;
        port = "ssh";
        filter = "sshd";
      };
    };
  };

  # SSH server
  services.openssh = {
    enable = true;
    settings = {
      PasswordAuthentication = false;
      PermitRootLogin = "no";
    };
  };

  # Automatic updates
  system.autoUpgrade = {
    enable = true;
    allowReboot = true;
    rebootWindow = {
      lower = "03:00";
      upper = "04:00";
    };
  };

  # Garbage collection
  nix.gc = {
    automatic = true;
    dates = "weekly";
    options = "--delete-older-than 30d";
  };

  # Enable flakes (optional but recommended)
  nix.settings.experimental-features = [ "nix-command" "flakes" ];

  # System version
  system.stateVersion = "24.11";
}
