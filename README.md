# Juniper Bible - NixOS Host Setup

NixOS server configuration for self-hosting Juniper Bible.

## One-Liner Install

Boot NixOS installer ISO, get network access, then:

```bash
curl -fsSL https://raw.githubusercontent.com/JuniperBible/Website.Server.JuniperBible.org/main/bootstrap.sh | sudo bash
```

Or fully automated (no prompts):

```bash
curl -fsSL https://raw.githubusercontent.com/JuniperBible/Website.Server.JuniperBible.org/main/bootstrap.sh | sudo bash -s -- /dev/sda "ssh-ed25519 AAAA..."
```

## After Reboot

```bash
ssh deploy@YOUR_SERVER_IP
deploy-juniper
```

Or from your dev machine:

```bash
make vps VPS=deploy@YOUR_SERVER_IP:/var/www/juniperbible
```

## What's Included

- **Caddy** - Automatic HTTPS, Brotli/Gzip pre-compression support
- **deploy user** - Unprivileged SSH access for deployments
- **deploy-juniper** - Script to pull and extract latest site
- **Firewall** - HTTP (80), HTTPS (443), SSH (22)
- **Auto-updates** - Weekly NixOS security updates
- **Nix garbage collection** - Automatic cleanup

## Files

| File | Purpose |
|------|---------|
| `bootstrap.sh` | One-liner installer (partitions, formats, installs) |
| `configuration.nix` | NixOS system configuration |
| `hardware-configuration.nix` | Template (replaced during install) |
| `install.sh` | Manual installer (if already partitioned) |
| `QUICKSTART.txt` | Copy-paste reference |

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
