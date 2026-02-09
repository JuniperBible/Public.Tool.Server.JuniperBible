package main

import (
	"fmt"
	"os"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/bootstrap"
	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/installer"
	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/upgrade"
	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/wizard"
)

var version = "dev"

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	switch os.Args[1] {
	case "bootstrap":
		bootstrap.Run(os.Args[2:])
	case "install":
		installer.Run(os.Args[2:])
	case "wizard", "setup":
		wizard.Run(os.Args[2:])
	case "upgrade":
		upgrade.Run(os.Args[2:])
	case "version", "--version", "-v":
		fmt.Printf("juniper-host %s\n", version)
	case "help", "--help", "-h":
		printUsage()
	default:
		fmt.Fprintf(os.Stderr, "Unknown command: %s\n", os.Args[1])
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`juniper-host - NixOS server setup for Juniper Bible

Usage:
  juniper-host <command> [options]

Commands:
  bootstrap    Full automated install (partition, format, install NixOS)
  install      Install NixOS to pre-mounted /mnt
  wizard       Interactive setup wizard (run after first boot)
  upgrade      Update configuration on local or remote host
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
  juniper-host upgrade`)
}
