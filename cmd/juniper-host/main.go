package main

import (
	"fmt"
	"os"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/bootstrap"
	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/installer"
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
  bootstrap    Full installation (partition, format, install NixOS)
  install      Install NixOS (requires pre-mounted /mnt)
  wizard       Interactive setup wizard (run after first boot)
  version      Show version

Bootstrap Options:
  --disk=DEVICE    Target disk (auto-detects if not specified)
  --ssh-key=KEY    SSH public key (prompts if not specified)
  --yes            Skip confirmation prompts

Examples:
  # Auto-detect disk, prompt for SSH key
  juniper-host bootstrap

  # Specify disk and SSH key
  juniper-host bootstrap --disk=/dev/vda --ssh-key="ssh-ed25519 AAAA..."

  # Full automation (no prompts)
  juniper-host bootstrap --disk=/dev/vda --ssh-key="ssh-ed25519 AAAA..." --yes`)
}
