package main

import (
	"fmt"
	"os"

	"github.com/JuniperBible/juniper-server/internal/bootstrap"
	"github.com/JuniperBible/juniper-server/internal/deploycmd"
	"github.com/JuniperBible/juniper-server/internal/installer"
	"github.com/JuniperBible/juniper-server/internal/upgrade"
	"github.com/JuniperBible/juniper-server/internal/wizard"
)

var version = "dev"

// commandHandlers maps commands to their handlers
var commandHandlers = map[string]func([]string){
	"bootstrap": bootstrap.Run,
	"install":   installer.Run,
	"wizard":    wizard.Run,
	"setup":     wizard.Run,
	"upgrade":   upgrade.Run,
	"deploy":    deploycmd.Run,
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	cmd := os.Args[1]
	args := os.Args[2:]

	// Check for handlers
	if handler, ok := commandHandlers[cmd]; ok {
		handler(args)
		return
	}

	// Built-in commands
	switch cmd {
	case "version", "--version", "-v":
		fmt.Printf("juniper-host %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", cmd)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`juniper-host - NixOS server setup and deployment for Juniper Bible

Usage:
  juniper-host <command> [options]

Commands:
  bootstrap    Full automated install (partition, format, install NixOS)
  install      Install NixOS to pre-mounted /mnt
  wizard       Interactive setup wizard (run after first boot)
  upgrade      Update configuration on local or remote host
  deploy       Deploy website with atomic delta sync
  version      Show version
  help         Show this help message

Bootstrap Options:
  --disk=DEVICE        Target disk (auto-detects if not specified)
  --ssh-key=KEY        SSH public key (prompts if not specified)
  --ssh-key-file=PATH  Path to SSH public key file (e.g., ~/.ssh/id_ed25519.pub)
  --yes                Skip all confirmation prompts
  --enthusiastic-yes   Auto-detect disk, skip confirmations, only prompt for SSH key

Upgrade Options:
  --host=HOST          Remote host (e.g., root@server or root@192.168.1.1)
  -i PATH              SSH identity file (optional)
  --yes                Skip confirmation prompts
  --config-only        Only update configuration, don't rebuild NixOS

Examples:
  # Auto-detect disk, prompt for SSH key
  juniper-host bootstrap

  # Quick install: auto-detect everything, just paste your SSH key
  juniper-host bootstrap --enthusiastic-yes

  # Use SSH key from file
  juniper-host bootstrap --enthusiastic-yes --ssh-key-file=~/.ssh/id_ed25519.pub

  # Specify disk and SSH key inline
  juniper-host bootstrap --disk=/dev/vda --ssh-key="ssh-ed25519 AAAA..."

  # Full automation (no prompts at all)
  juniper-host bootstrap --disk=/dev/vda --ssh-key="ssh-ed25519 AAAA..." --yes

  # Upgrade remote server
  juniper-host upgrade --host=root@your-server

  # Upgrade local NixOS (run on the server itself)
  juniper-host upgrade

Deploy Options:
  --config=PATH        Path to deploy.toml (default: deploy.toml)
  --release=ID         Release ID (default: auto-generated timestamp-hash)
  --dry-run            Show what would be deployed without deploying
  --full               Upload all files instead of delta
  --no-build           Skip Hugo build (use existing public/ directory)

Deploy Examples:
  # Deploy to local releases directory
  juniper-host deploy local

  # Deploy to production VPS
  juniper-host deploy prod

  # Preview what would be deployed
  juniper-host deploy prod --dry-run

  # List releases on production
  juniper-host deploy list prod

  # Rollback to previous release
  juniper-host deploy rollback prod`)
}
