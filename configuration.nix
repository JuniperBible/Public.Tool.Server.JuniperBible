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

  # Boot loader
  boot.loader.systemd-boot.enable = true;
  boot.loader.efi.canTouchEfiVariables = true;

  # Networking
  networking.networkmanager.enable = true;
  networking.firewall.allowedTCPPorts = [ 22 80 443 ];

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

      REPO="https://github.com/FocuswithJustin/Website.Public.JuniperBible.org"
      RELEASE_URL="$REPO/releases/latest/download/site.tar.xz"
      DEPLOY_DIR="/var/www/juniperbible"
      TEMP_DIR=$(mktemp -d)

      echo "Downloading latest release..."
      curl -fsSL "$RELEASE_URL" -o "$TEMP_DIR/site.tar.xz" || {
        echo "No release found. Building from source..."
        cd "$TEMP_DIR"
        git clone --depth 1 "$REPO" repo
        cd repo
        nix-shell --run "make build"
        tar -C public -cf - . | xz -T0 -9 > "$TEMP_DIR/site.tar.xz"
      }

      echo "Extracting to $DEPLOY_DIR..."
      rm -rf "$DEPLOY_DIR"/*
      mkdir -p "$DEPLOY_DIR"
      tar -xJf "$TEMP_DIR/site.tar.xz" -C "$DEPLOY_DIR"

      echo "Cleaning up..."
      rm -rf "$TEMP_DIR"

      echo "Done! Site deployed to $DEPLOY_DIR"
    '';
  };

  # Add deploy script to PATH
  environment.shellAliases = {
    deploy-juniper = "sudo /etc/deploy-juniper.sh";
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
    allowReboot = false;
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
