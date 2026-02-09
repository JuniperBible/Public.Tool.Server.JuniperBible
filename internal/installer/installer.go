package installer

import (
	"fmt"
	"os"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/common"
)

// Run executes the install command (requires pre-mounted /mnt)
func Run(args []string) {
	// Check root
	if !common.IsRoot() {
		common.Error("Must be run as root")
		fmt.Println("Usage: sudo juniper-host install")
		os.Exit(1)
	}

	common.Header("Juniper Bible - NixOS Host Installation")

	// Check if /mnt is mounted
	if !common.IsMounted("/mnt") {
		common.Error("/mnt is not mounted.")
		fmt.Println()
		fmt.Println("Please partition and mount your disk first:")
		fmt.Println()
		fmt.Println("  # For /dev/sda (or /dev/vda on cloud VPS):")
		fmt.Println("  parted /dev/sda -- mklabel gpt")
		fmt.Println("  parted /dev/sda -- mkpart ESP fat32 1MB 512MB")
		fmt.Println("  parted /dev/sda -- set 1 esp on")
		fmt.Println("  parted /dev/sda -- mkpart primary 512MB 100%")
		fmt.Println("  mkfs.fat -F 32 -n boot /dev/sda1")
		fmt.Println("  mkfs.ext4 -L nixos /dev/sda2")
		fmt.Println("  mount /dev/sda2 /mnt")
		fmt.Println("  mkdir -p /mnt/boot")
		fmt.Println("  mount /dev/sda1 /mnt/boot")
		fmt.Println()
		os.Exit(1)
	}

	// Check if /mnt/boot is mounted
	if !common.IsMounted("/mnt/boot") {
		common.Error("/mnt/boot is not mounted.")
		fmt.Println("Please mount your boot partition: mount /dev/sda1 /mnt/boot")
		os.Exit(1)
	}

	// Step 1: Generate hardware config
	common.Info("Generating hardware configuration...")
	if err := common.Run("nixos-generate-config", "--root", "/mnt"); err != nil {
		common.Error(fmt.Sprintf("Failed to generate hardware config: %v", err))
		os.Exit(1)
	}

	// Step 2: Download configuration
	fmt.Println()
	common.Info("Downloading Juniper Bible configuration...")
	configURL := common.RepoBase + "/configuration.nix"
	if err := common.DownloadFile(configURL, "/mnt/etc/nixos/configuration.nix"); err != nil {
		common.Error(fmt.Sprintf("Failed to download configuration: %v", err))
		os.Exit(1)
	}

	// Step 3: Install NixOS
	fmt.Println()
	common.Info("Installing NixOS...")
	common.Warning("This takes 10-30 minutes on VPS (downloading packages from cache.nixos.org)")
	common.Info("Progress dots will appear every 5 seconds. Do NOT interrupt.")
	fmt.Println()
	if err := common.RunWithProgress("nixos-install", "--no-root-passwd"); err != nil {
		common.Error(fmt.Sprintf("Installation failed: %v", err))
		os.Exit(1)
	}

	// Done
	fmt.Println()
	common.Header("Installation complete!")
	fmt.Println("IMPORTANT: Before rebooting, you should:")
	fmt.Println()
	fmt.Println("1. Edit /mnt/etc/nixos/configuration.nix to add your SSH key:")
	fmt.Println("   nano /mnt/etc/nixos/configuration.nix")
	fmt.Println()
	fmt.Println("   Find this line and add your key:")
	fmt.Println(`   users.users.deploy.openssh.authorizedKeys.keys = [`)
	fmt.Println(`     "ssh-ed25519 AAAA... your-key-here"`)
	fmt.Println(`   ];`)
	fmt.Println()
	fmt.Println("2. Set your domain (if not juniperbible.org):")
	fmt.Println(`   services.caddy.virtualHosts."your-domain.com".extraConfig = ...`)
	fmt.Println()
	fmt.Println("3. Rebuild to apply changes:")
	fmt.Println("   nixos-install --no-root-passwd")
	fmt.Println()
	fmt.Println("4. Reboot:")
	fmt.Println("   reboot")
	fmt.Println()
	fmt.Println("After reboot, SSH in as 'deploy' user and run:")
	fmt.Println("   deploy-juniper")
	fmt.Println()
}
