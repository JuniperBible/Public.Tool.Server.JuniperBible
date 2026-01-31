# Juniper Bible - NixOS Host Setup

NixOS server configuration for self-hosting Juniper Bible.

## Quick Install

Download the latest binary for your platform from [Releases](https://github.com/JuniperBible/Website.Server.JuniperBible.org/releases), then:

```bash
# Make executable
chmod +x juniper-host-linux-amd64

# Run bootstrap (auto-detects disk, prompts for SSH key)
sudo ./juniper-host-linux-amd64 bootstrap
```

### One-Liner (Linux amd64)

```bash
curl -fsSL https://github.com/JuniperBible/Website.Server.JuniperBible.org/releases/latest/download/juniper-host-linux-amd64.tar.gz | tar -xzf - && sudo ./juniper-host-linux-amd64 bootstrap
```

### Fully Automated

```bash
sudo ./juniper-host-linux-amd64 bootstrap --disk=/dev/vda --ssh-key="ssh-ed25519 AAAA..." --yes
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
| `--disk=DEVICE` | Target disk (auto-detects if not specified) |
| `--ssh-key=KEY` | SSH public key (prompts if not specified) |
| `--yes` | Skip confirmation prompts |

## After Reboot

SSH in and the setup wizard runs automatically:

```bash
ssh deploy@YOUR_SERVER_IP
```

The wizard guides you through:
1. Hostname configuration
2. Domain for HTTPS
3. SSH keys
4. Site deployment

## Supported Platforms

Pre-built binaries are available for:

| OS | Architecture |
|----|--------------|
| Linux | amd64, arm64, 386, armv7 |
| macOS | amd64 (Intel), arm64 (Apple Silicon) |
| Windows | amd64, arm64 |
| FreeBSD | amd64, arm64 |

## What's Included

- **Caddy** - Automatic HTTPS, Brotli/Gzip pre-compression support
- **deploy user** - Unprivileged SSH access for deployments
- **deploy-juniper** - Script to pull and extract latest site
- **Firewall** - HTTP (80), HTTPS (443), SSH (22)
- **Auto-updates** - Weekly NixOS security updates
- **Nix garbage collection** - Automatic cleanup

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

## Customization

After installation, edit `/etc/nixos/configuration.nix`:

```bash
sudo nano /etc/nixos/configuration.nix
sudo nixos-rebuild switch
```

### Change Domain

```nix
services.caddy.virtualHosts."your-domain.com".extraConfig = ''
  ...
'';
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
