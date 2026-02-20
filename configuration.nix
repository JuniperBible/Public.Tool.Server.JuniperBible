{ config, pkgs, ... }:

{
  imports = [
    ./hardware-configuration.nix
  ];

  # ==========================================================================
  # CUSTOMIZE THESE VALUES
  # ==========================================================================

  networking.hostName = "juniperbible";

  # TLS Mode Options:
  #   1 = ACME HTTP-01 (requires DNS pointing directly to server)
  #   2 = ACME DNS-01 via Cloudflare (works behind proxy, requires API token)
  #   3 = Custom certificate (provide cert/key paths)
  #   4 = HTTP only (no TLS, for localhost/testing)
  #   5 = Self-signed (default, works everywhere, browser warning)
  #
  # TLS_MODE = "5";
  # TLS_DOMAIN = "juniperbible.org";
  # TLS_CF_API_TOKEN = "";  # For mode 2 only
  # TLS_CERT_PATH = "";     # For mode 3 only
  # TLS_KEY_PATH = "";      # For mode 3 only

  # Add your SSH public key here (same key for deploy and root users)
  users.users.deploy.openssh.authorizedKeys.keys = [
    # "ssh-ed25519 AAAA... your-key-here"
  ];

  users.users.root.openssh.authorizedKeys.keys = [
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
    # Network hardening (IPv4)
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
    # Network hardening (IPv6)
    "net.ipv6.conf.all.accept_redirects" = 0;
    "net.ipv6.conf.default.accept_redirects" = 0;
    "net.ipv6.conf.all.accept_source_route" = 0;
    "net.ipv6.conf.default.accept_source_route" = 0;
    "net.ipv6.conf.all.accept_ra" = 0;
    "net.ipv6.conf.default.accept_ra" = 0;
    # Kernel hardening
    "kernel.yama.ptrace_scope" = 1;
    "kernel.kptr_restrict" = 2;
    "kernel.dmesg_restrict" = 1;
    "kernel.perf_event_paranoid" = 3;
    "kernel.sysrq" = 0;
    "kernel.randomize_va_space" = 2;
    # Filesystem hardening
    "fs.suid_dumpable" = 0;
    "fs.protected_symlinks" = 1;
    "fs.protected_hardlinks" = 1;
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

  # Caddy web server with writable Caddyfile
  # The Caddyfile is stored in /var/lib/caddy/Caddyfile (writable)
  # and symlinked from /etc/caddy/Caddyfile for convenience
  services.caddy = {
    enable = true;
    configFile = "/var/lib/caddy/Caddyfile";
  };

  # Create default Caddyfile in writable location
  # This file can be modified by the setup wizard
  systemd.tmpfiles.rules = [
    "d /var/lib/caddy 0755 caddy caddy -"
    "d /var/www/juniperbible 0755 deploy caddy -"
  ];

  # Initialize default Caddyfile on first boot
  system.activationScripts.initCaddyfile = ''
    if [ ! -f /var/lib/caddy/Caddyfile ]; then
      cat > /var/lib/caddy/Caddyfile << 'CADDYEOF'
# Juniper Bible - Caddy Configuration
# TLS Mode: self-signed (default)
# This file will be replaced by the setup wizard
{
  log {
    level ERROR
  }
  # HTTP/3 support (QUIC)
  servers {
    protocols h1 h2 h3
  }
  # Global performance settings
  admin off
}

# Shared site configuration
(site_config) {
  root * /var/www/juniperbible/current
  encode gzip

  # Static redirects (301)
  @religion path /religion/*
  redir @religion /bible/drc/isa/42/ 301

  @licenses path /licenses/*
  redir @licenses /license/ 301

  # SPA-style rewrites for compare page clean URLs
  @compare_spa {
    path_regexp ^/bible/compare/[^/]+/[^/]+/[^/]+
  }
  rewrite @compare_spa /bible/compare/index.html

  file_server {
    precompressed br gzip
  }

  # Cache static assets (1 year, immutable for fingerprinted assets)
  @static {
    path *.css *.js *.woff2 *.png *.jpg *.svg *.ico
  }
  header @static Cache-Control "public, max-age=31536000, immutable"

  # Cache Bible archives (1 year, immutable - content rarely changes)
  @archives {
    path /bible-archives/*
  }
  header @archives Cache-Control "public, max-age=31536000, immutable"

  # Cache Bible pages (1 day, stale-while-revalidate for fast loads)
  @bible {
    path /bible/*
  }
  header @bible Cache-Control "public, max-age=86400, stale-while-revalidate=604800"

  # Security headers
  header {
    X-Content-Type-Options nosniff
    X-Frame-Options DENY
    Referrer-Policy strict-origin-when-cross-origin
    Permissions-Policy "camera=(), microphone=(), geolocation=()"
  }
}

# HTTPS with self-signed certificate
:443 {
  tls internal
  import site_config
}

# HTTP - serve content (for Cloudflare proxy) or redirect to HTTPS
:80 {
  import site_config
}
CADDYEOF
      chown caddy:caddy /var/lib/caddy/Caddyfile
      chmod 644 /var/lib/caddy/Caddyfile
    fi
  '';

  # Deploy user (for SSH deployments)
  users.users.deploy = {
    isNormalUser = true;
    home = "/home/deploy";
    extraGroups = [ "caddy" ];
    shell = pkgs.bash;
  };

  # Web directory is created in systemd.tmpfiles.rules above

  # Deployment script
  environment.etc."deploy-juniper.sh" = {
    mode = "0755";
    text = ''
      #!/usr/bin/env bash
      set -euo pipefail

      RELEASE_URL="https://github.com/JuniperBible/Website.Server.JuniperBible.org/releases/latest/download/site.tar.xz"
      DEPLOY_DIR="/var/www/juniperbible"
      TEMP_DIR=$(mktemp -d)

      # Safety check: ensure DEPLOY_DIR is set and looks valid
      if [[ -z "$DEPLOY_DIR" || "$DEPLOY_DIR" != /var/www/* ]]; then
        echo "ERROR: Invalid DEPLOY_DIR: $DEPLOY_DIR"
        exit 1
      fi

      # Variables for cleanup
      STAGING_DIR=""

      # Cleanup function
      cleanup() {
        rm -rf "$TEMP_DIR" 2>/dev/null || true
        [[ -n "$STAGING_DIR" && -d "$STAGING_DIR" ]] && rm -rf "$STAGING_DIR" 2>/dev/null || true
      }
      trap cleanup EXIT INT TERM

      echo "Downloading latest release..."
      if ! curl -fsSL "$RELEASE_URL" -o "$TEMP_DIR/site.tar.xz"; then
        echo "ERROR: Failed to download release from $RELEASE_URL"
        echo "Please check that a release exists with site.tar.xz attached."
        exit 1
      fi

      echo "Extracting to $DEPLOY_DIR..."
      # Use atomic replacement: extract to temp, then move
      STAGING_DIR=$(mktemp -d -p /var/www)
      tar -xJf "$TEMP_DIR/site.tar.xz" -C "$STAGING_DIR" --no-same-owner --no-symlinks
      # Sync to ensure extraction is complete before swap
      sync
      # Atomic swap: remove old and move new
      rm -rf "$DEPLOY_DIR"
      mv "$STAGING_DIR" "$DEPLOY_DIR"
      STAGING_DIR=""  # Clear so cleanup doesn't try to remove new deploy dir

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

      # Lock file to prevent concurrent wizard runs
      LOCK_FILE="/tmp/juniper-setup.lock"
      exec 200>"$LOCK_FILE"
      if ! flock -n 200; then
        echo "Setup wizard is already running in another session."
        exit 0
      fi

      # Gather system info
      SYS_HOSTNAME=$(hostname)
      SYS_IP=$(hostname -i 2>/dev/null | awk '{print $1}' || ip -4 addr show | grep -oP '(?<=inet\s)\d+(\.\d+){3}' | grep -v '^127\.' | head -1 || echo "N/A")
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

      # Input validation functions
      validate_hostname() {
        local h="$1"
        [[ ''${#h} -gt 0 && ''${#h} -le 63 && "$h" =~ ^[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$ ]]
      }
      validate_domain() {
        local d="$1"
        [[ "$d" == "localhost" ]] && return 0
        [[ ''${#d} -gt 0 && ''${#d} -le 253 && "$d" =~ ^([a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]*[a-zA-Z0-9])?$ ]]
      }
      escape_sed() {
        # Escape special chars for sed: backslash first, then others
        printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/[\/&|]/\\&/g' -e 's/"/\\"/g'
      }

      # Step 1: Hostname
      clear
      echo "Step 1/4: Hostname"
      echo ""
      current_hostname=$(hostname)
      echo "Current: $current_hostname"
      for attempt in 1 2 3 4 5; do
        read -p "New hostname (Enter to keep): " new_hostname
        new_hostname="''${new_hostname:-$current_hostname}"
        if validate_hostname "$new_hostname"; then
          break
        fi
        echo "Invalid hostname. Use alphanumerics and hyphens (1-63 chars)."
        [[ $attempt -eq 5 ]] && { echo "Too many invalid attempts."; exit 1; }
      done

      # Step 2: Domain
      clear
      echo "Step 2/5: Domain"
      echo ""
      echo "Enter domain for HTTPS (e.g., juniperbible.org)"
      echo ""
      for attempt in 1 2 3 4 5; do
        read -p "Domain: " domain
        domain="''${domain:-localhost}"
        if validate_domain "$domain"; then
          break
        fi
        echo "Invalid domain format."
        [[ $attempt -eq 5 ]] && { echo "Too many invalid attempts."; exit 1; }
      done

      # Step 3: TLS Mode
      clear
      echo "Step 3/5: TLS Certificate Mode"
      echo ""
      echo "How should HTTPS certificates be handled?"
      echo ""
      echo "  1) ACME HTTP-01  - Auto cert, requires DNS pointing directly to this server"
      echo "  2) ACME DNS-01   - Auto cert via Cloudflare DNS (works behind proxy)"
      echo "  3) Custom cert   - Provide your own certificate files"
      echo "  4) HTTP only     - No HTTPS (for testing only)"
      echo "  5) Self-signed   - Works everywhere, browser shows warning (default)"
      echo ""
      read -p "TLS mode [5]: " tls_mode
      tls_mode="''${tls_mode:-5}"

      cf_api_token=""
      cert_path=""
      key_path=""

      case "$tls_mode" in
        1) echo "Using ACME HTTP-01 challenge" ;;
        2)
          echo ""
          echo "Enter your Cloudflare API token (needs Zone:DNS:Edit permission):"
          read -p "CF API Token: " cf_api_token
          if [[ -z "$cf_api_token" ]]; then
            echo "API token required for DNS-01. Falling back to self-signed."
            tls_mode="5"
          fi
          ;;
        3)
          echo ""
          read -p "Certificate path: " cert_path
          read -p "Key path: " key_path
          if [[ ! -f "$cert_path" || ! -f "$key_path" ]]; then
            echo "Certificate files not found. Falling back to self-signed."
            tls_mode="5"
          fi
          ;;
        4) echo "Using HTTP only (no TLS)" ;;
        5|*) tls_mode="5"; echo "Using self-signed certificate" ;;
      esac

      # Step 4: SSH Keys
      clear
      echo "Step 4/5: SSH Keys"
      echo ""
      echo "Paste SSH public keys (one per line, empty line to finish)"
      echo ""
      ssh_keys=()
      while true; do
        read -p "Key: " key
        [[ -z "$key" ]] && break
        # Validate SSH key format (must be valid type + base64 data)
        if [[ "$key" =~ ^(ssh-rsa|ssh-ed25519|ecdsa-sha2-nistp[0-9]+)[[:space:]]+[A-Za-z0-9+/]+=*([[:space:]].+)?$ ]]; then
          ssh_keys+=("$key")
          echo "✓ Added"
        else
          echo "Invalid format (should be: ssh-ed25519/ssh-rsa/ecdsa-* followed by base64 key)"
        fi
      done

      # Step 5: Deploy now?
      clear
      echo "Step 5/5: Deploy Site"
      echo ""
      read -p "Deploy Juniper Bible now? [Y/n] " -n 1 -r deploy_now
      echo
      deploy_now="''${deploy_now:-y}"

      # TLS mode display names
      tls_mode_name="Self-signed"
      case "$tls_mode" in
        1) tls_mode_name="ACME HTTP-01" ;;
        2) tls_mode_name="ACME DNS-01 (Cloudflare)" ;;
        3) tls_mode_name="Custom certificate" ;;
        4) tls_mode_name="HTTP only" ;;
        5) tls_mode_name="Self-signed" ;;
      esac

      # Confirm
      clear
      echo "Configuration Summary"
      echo ""
      echo "  Hostname: $new_hostname"
      echo "  Domain:   $domain"
      echo "  TLS Mode: $tls_mode_name"
      echo "  SSH Keys: ''${#ssh_keys[@]}"
      echo ""
      read -p "Apply? [Y/n] " -n 1 -r confirm
      echo
      [[ ! $confirm =~ ^[Yy]?$ ]] && { echo "Cancelled."; exit 1; }

      # Apply (running as root, no sudo needed)
      echo "Applying configuration..."
      if ! cp "$NIXOS_CONFIG" "''${NIXOS_CONFIG}.backup"; then
        echo "ERROR: Failed to create backup"
        exit 1
      fi

      # Escape inputs for sed
      escaped_hostname=$(escape_sed "$new_hostname")

      sed -i "s/networking.hostName = \".*\"/networking.hostName = \"$escaped_hostname\"/" "$NIXOS_CONFIG"

      if [[ ''${#ssh_keys[@]} -gt 0 ]]; then
        keys_nix=""
        for key in "''${ssh_keys[@]}"; do
          escaped_key=$(escape_sed "$key")
          keys_nix+="    \"$escaped_key\"\n"
        done
        sed -i '/authorizedKeys.keys = \[/,/\];/{/authorizedKeys.keys = \[/!{/\];/!d}}' "$NIXOS_CONFIG"
        sed -i "s|authorizedKeys.keys = \[|authorizedKeys.keys = [\n$keys_nix|" "$NIXOS_CONFIG"
      fi

      # Generate Caddyfile based on TLS mode
      echo "Generating Caddyfile..."
      CADDYFILE="/var/lib/caddy/Caddyfile"

      # Shared site configuration snippet (imported by each server block)
      site_config_snippet='(site_config) {
  root * /var/www/juniperbible/current
  encode gzip

  # Static redirects (301) - matches _redirects
  @religion path /religion/*
  redir @religion /bible/drc/isa/42/ 301

  @licenses path /licenses/*
  redir @licenses /license/ 301

  # SPA-style rewrites for compare page clean URLs
  # Matches: /bible/compare/{bibles}/{book}/{chapter}[/{verse}][/{mode}]
  @compare_spa {
    path_regexp ^/bible/compare/[^/]+/[^/]+/[^/]+
  }
  rewrite @compare_spa /bible/compare/index.html

  file_server {
    precompressed br gzip
  }

  # Cache static assets (1 year, immutable)
  @static {
    path *.css *.js *.woff2 *.png *.jpg *.svg *.ico
  }
  header @static Cache-Control "public, max-age=31536000, immutable"

  # Cache Bible archives (1 year, immutable)
  @archives {
    path /bible-archives/*
  }
  header @archives Cache-Control "public, max-age=31536000, immutable"

  # Cache Bible pages (1 day + stale-while-revalidate)
  @bible {
    path /bible/*
  }
  header @bible Cache-Control "public, max-age=86400, stale-while-revalidate=604800"

  header {
    X-Content-Type-Options nosniff
    X-Frame-Options DENY
    Referrer-Policy strict-origin-when-cross-origin
    Permissions-Policy "camera=(), microphone=(), geolocation=()"
  }
}'

      case "$tls_mode" in
        1)
          # ACME HTTP-01
          cat > "$CADDYFILE" << CADDYEOF
# Juniper Bible - TLS Mode: ACME HTTP-01
{
  log {
    level ERROR
  }
}

$site_config_snippet

$domain {
  import site_config
  header Strict-Transport-Security "max-age=31536000; includeSubDomains"
}
CADDYEOF
          ;;
        2)
          # ACME DNS-01 via Cloudflare
          cat > "$CADDYFILE" << CADDYEOF
# Juniper Bible - TLS Mode: ACME DNS-01 (Cloudflare)
{
  log {
    level ERROR
  }
}

$site_config_snippet

$domain {
  tls {
    dns cloudflare $cf_api_token
  }
  import site_config
  header Strict-Transport-Security "max-age=31536000; includeSubDomains"
}
CADDYEOF
          ;;
        3)
          # Custom certificate
          cat > "$CADDYFILE" << CADDYEOF
# Juniper Bible - TLS Mode: Custom Certificate
{
  log {
    level ERROR
  }
}

$site_config_snippet

$domain {
  tls $cert_path $key_path
  import site_config
  header Strict-Transport-Security "max-age=31536000; includeSubDomains"
}
CADDYEOF
          ;;
        4)
          # HTTP only
          cat > "$CADDYFILE" << CADDYEOF
# Juniper Bible - TLS Mode: HTTP Only
{
  log {
    level ERROR
  }
}

$site_config_snippet

:80 {
  import site_config
}
CADDYEOF
          ;;
        5|*)
          # Self-signed (default) - serves HTTP for Cloudflare proxy
          cat > "$CADDYFILE" << CADDYEOF
# Juniper Bible - TLS Mode: Self-signed
# HTTP is served without redirect (for Cloudflare proxy)
# Direct HTTPS uses self-signed certificate
{
  log {
    level ERROR
  }
}

$site_config_snippet

# HTTPS with self-signed certificate (direct access)
$domain, :443 {
  tls internal
  import site_config
}

# HTTP - serve content (for Cloudflare proxy)
:80 {
  import site_config
}
CADDYEOF
          ;;
      esac

      chown caddy:caddy "$CADDYFILE"
      chmod 644 "$CADDYFILE"
      echo "✓ Caddyfile generated"

      echo "Rebuilding NixOS..."
      if nixos-rebuild switch; then
        echo "✓ Done"
      else
        echo "✗ Failed - restoring backup..."
        cp "''${NIXOS_CONFIG}.backup" "$NIXOS_CONFIG" && echo "✓ Backup restored" || echo "✗ Restore failed"
        exit 1
      fi

      touch "$SETUP_DONE_FLAG"
      chmod 644 "$SETUP_DONE_FLAG"

      if [[ $deploy_now =~ ^[Yy]?$ ]]; then
        echo "Deploying site..."
        /etc/deploy-juniper.sh || echo "Deploy failed - run 'deploy-juniper' to retry"
      fi

      echo ""
      echo "Setup Complete!"
      echo ""
      [[ "$domain" != "localhost" ]] && echo "  https://$domain" || echo "  http://localhost"
      echo "  ssh deploy@$SYS_IP"
      echo "  ssh root@$SYS_IP  (for admin tasks)"
      echo ""
    '';
  };

  # Add scripts to PATH
  environment.shellAliases = {
    deploy-juniper = "sudo /etc/deploy-juniper.sh";
    setup-wizard = "/etc/setup-wizard.sh";
  };

  # Run setup wizard on first login for root user
  programs.bash.interactiveShellInit = ''
    if [[ ! -f /etc/juniper-setup-complete ]] && [[ $USER == "root" ]]; then
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
      PermitRootLogin = "prohibit-password";  # Allow key-based root login only
      PermitEmptyPasswords = false;
      X11Forwarding = false;
      AllowTcpForwarding = false;
      AllowAgentForwarding = false;
      MaxAuthTries = 3;
      MaxSessions = 3;
      ClientAliveInterval = 300;
      ClientAliveCountMax = 2;
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
