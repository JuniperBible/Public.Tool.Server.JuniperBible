# Juniper Bible - NixOS Host Setup

NixOS server configuration for self-hosting Juniper Bible.

## Quickstart

### 1. Boot NixOS Installer

Download the [NixOS minimal ISO](https://nixos.org/download/#nixos-iso) and boot your server from it.

### 2. Get Network Access

```bash
# For DHCP (most cases)
sudo systemctl start network-manager
nmcli device wifi connect "SSID" password "PASSWORD"  # if WiFi

# Verify connectivity
ping -c 3 github.com
```

### 3. Run Bootstrap

```bash
# Download and run (auto-detects disk)
curl -fsSL https://github.com/JuniperBible/Website.Server.JuniperBible.org/releases/latest/download/juniper-host-linux-amd64.tar.gz | tar -xzf -

# Quick install: auto-detect everything, just paste your SSH key when prompted
sudo ./juniper-host-linux-amd64 bootstrap --enthusiastic-yes

# Or with SSH key file (no prompts needed)
sudo ./juniper-host-linux-amd64 bootstrap --enthusiastic-yes --ssh-key-file=~/.ssh/id_ed25519.pub
```

### 4. Reboot & Configure

After reboot, SSH in as `root` to run the setup wizard:

```bash
ssh root@YOUR_SERVER_IP
```

Then use the `deploy` user for ongoing deployments.

## Deployment Options

### Option A: Cloud VPS (Vultr, Hetzner, DigitalOcean)

1. Create a new VPS and boot the NixOS ISO via console/VNC
2. Run the bootstrap one-liner (disk is usually `/dev/vda`)
3. After reboot, complete the setup wizard via SSH

```bash
# Fully automated (no prompts)
sudo ./juniper-host-linux-amd64 bootstrap \
  --disk=/dev/vda \
  --ssh-key="ssh-ed25519 AAAA..." \
  --yes
```

### Option B: Dedicated Server / Bare Metal

1. Boot from NixOS USB/ISO
2. Run bootstrap (disk is usually `/dev/sda` or `/dev/nvme0n1`)
3. After reboot, complete the setup wizard

```bash
# Interactive mode
sudo ./juniper-host-linux-amd64 bootstrap
```

### Option C: Manual Installation

If you prefer to partition manually:

```bash
# Partition (example for /dev/sda)
parted /dev/sda -- mklabel gpt
parted /dev/sda -- mkpart ESP fat32 1MB 512MB
parted /dev/sda -- set 1 esp on
parted /dev/sda -- mkpart primary 512MB 100%

# Format
mkfs.fat -F 32 -n boot /dev/sda1
mkfs.ext4 -L nixos /dev/sda2

# Mount
mount /dev/sda2 /mnt
mkdir -p /mnt/boot
mount /dev/sda1 /mnt/boot

# Install
sudo ./juniper-host-linux-amd64 install
```

## Commands

| Command | Description |
|---------|-------------|
| `bootstrap` | Full install (partition, format, install NixOS) |
| `install` | Install NixOS (requires pre-mounted /mnt) |
| `wizard` | Interactive setup wizard (run after first boot) |
| `version` | Show version |

## Bootstrap Options

| Option | Description |
|--------|-------------|
| `--disk=DEVICE` | Target disk (auto-detects: vda, sda, nvme0n1, xvda) |
| `--ssh-key=KEY` | SSH public key (prompts if not specified) |
| `--ssh-key-file=PATH` | Path to SSH public key file (e.g., ~/.ssh/id_ed25519.pub) |
| `--yes` | Skip all confirmation prompts |
| `--enthusiastic-yes` | Auto-detect disk, skip confirmations, only prompt for SSH key |

## Post-Installation

### Setup Wizard

The wizard runs automatically on first SSH login as root and configures:

1. **Hostname** - Server name
2. **Domain** - For Caddy web server
3. **TLS Mode** - Certificate handling (see below)
4. **SSH Keys** - For the `deploy` and `root` users
5. **Site Deployment** - Downloads and extracts Juniper Bible

### TLS Certificate Modes

| Mode | Description | Use Case |
|------|-------------|----------|
| 1 - ACME HTTP-01 | Auto cert via HTTP challenge | DNS points directly to server |
| 2 - ACME DNS-01 | Auto cert via Cloudflare DNS | Behind proxy (Cloudflare, etc.) |
| 3 - Custom cert | Provide your own cert/key | Enterprise, existing certs |
| 4 - HTTP only | No HTTPS | Local testing only |
| 5 - Self-signed | Auto-generated, browser warning | **Default** - works everywhere |

### Manual Site Deployment

```bash
# Deploy latest release
deploy-juniper

# Or via make from your dev machine
make deploy-vps VPS=deploy@your-server:/var/www/juniperbible
```

### Updating the Site

```bash
ssh deploy@your-server
deploy-juniper
```

## Server Architecture

```
┌─────────────────────────────────────────────────────┐
│                    NixOS Server                      │
├─────────────────────────────────────────────────────┤
│  Caddy (reverse proxy + auto HTTPS)                 │
│    ├── Brotli/Gzip pre-compression                  │
│    ├── Security headers                             │
│    └── Static file serving                          │
├─────────────────────────────────────────────────────┤
│  /var/www/juniperbible                              │
│    ├── Bible HTML pages                             │
│    ├── CSS/JS assets                                │
│    └── Bible archives (br/gz/xz/json)               │
├─────────────────────────────────────────────────────┤
│  Services                                           │
│    ├── SSH (port 22) - key-only                     │
│    ├── HTTP (port 80) - redirects to HTTPS          │
│    └── HTTPS (port 443) - auto Let's Encrypt        │
├─────────────────────────────────────────────────────┤
│  Maintenance                                        │
│    ├── Auto-updates (weekly)                        │
│    └── Garbage collection (30 days)                 │
└─────────────────────────────────────────────────────┘
```

## Supported Platforms

Pre-built binaries are available for:

| OS | Architecture |
|----|--------------|
| Linux | amd64, arm64, 386, armv7 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) |
| Windows | amd64, arm64 |
| FreeBSD | amd64, arm64 |

## Customization

After installation, edit `/etc/nixos/configuration.nix`:

```bash
sudo nano /etc/nixos/configuration.nix
sudo nixos-rebuild switch
```

### Change Domain or TLS Mode

Edit `/etc/caddy/Caddyfile` directly or re-run the wizard:

```bash
# Edit Caddyfile
sudo nano /etc/caddy/Caddyfile
sudo systemctl reload caddy

# Or delete the setup flag and re-run wizard
sudo rm /etc/juniper-setup-complete
/etc/setup-wizard.sh
```

### Add SSH Keys

```nix
users.users.deploy.openssh.authorizedKeys.keys = [
  "ssh-ed25519 AAAA..."
  "ssh-rsa AAAA..."
];
```

### Change Hostname

```nix
networking.hostName = "your-hostname";
```

## Troubleshooting

### Bootstrap appears to hang during installation

This is normal. The `nixos-install` step takes **10-30 minutes** on VPS providers due to:
- Downloading packages from cache.nixos.org
- Building NixOS system configuration

Progress dots will appear every 5 seconds. Do not interrupt the process.

### Can't SSH after reboot

- Verify your SSH key was added during bootstrap
- Check firewall: `sudo iptables -L`
- Verify SSH service: `sudo systemctl status sshd`

### Caddy not serving HTTPS

- Ensure ports 80/443 are open on your firewall/VPS provider
- Check Caddy logs: `sudo journalctl -u caddy`
- For ACME HTTP-01: Verify DNS points directly to server IP (not proxied)
- For ACME DNS-01: Verify Cloudflare API token has Zone:DNS:Edit permission
- For self-signed: Browser will show certificate warning (this is normal)

### Site not loading

- Check site files exist: `ls /var/www/juniperbible`
- Re-deploy: `deploy-juniper`
- Check Caddy config: `sudo caddy validate --config /etc/caddy/Caddyfile`

## Building from Source

```bash
# Build for current platform
make build

# Build for all platforms
make release-local

# Install to /usr/local/bin
sudo make install
```

## Creating a Release

Push a tag to trigger the GitHub Actions release workflow:

```bash
git tag v1.0.0
git push origin v1.0.0
```

## Legacy Shell Scripts

The original shell scripts are still available in the `legacy/` directory for reference.
