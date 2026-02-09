package bootstrap

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/common"
)

// Run executes the bootstrap command
func Run(args []string) {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	disk := fs.String("disk", "", "Target disk (auto-detect if not specified)")
	sshKey := fs.String("ssh-key", "", "SSH public key")
	sshKeyFile := fs.String("ssh-key-file", "", "Path to SSH public key file")
	yes := fs.Bool("yes", false, "Skip confirmation prompts")
	enthusiasticYes := fs.Bool("enthusiastic-yes", false, "Auto-detect everything, only prompt for SSH key if not provided")
	fs.Parse(args)

	// --enthusiastic-yes implies --yes for disk confirmation
	if *enthusiasticYes {
		*yes = true
	}

	// Read SSH key from file if provided
	if *sshKeyFile != "" && *sshKey == "" {
		data, err := os.ReadFile(*sshKeyFile)
		if err != nil {
			common.Error(fmt.Sprintf("Failed to read SSH key file: %v", err))
			os.Exit(1)
		}
		keyStr := strings.TrimSpace(string(data))
		sshKey = &keyStr
	}

	// Check root
	if !common.IsRoot() {
		common.Error("Must be run as root")
		fmt.Println("Usage: sudo juniper-host bootstrap")
		os.Exit(1)
	}

	// Auto-detect disk if not specified
	targetDisk := *disk
	if targetDisk == "" {
		targetDisk = common.DetectDisk()
		if targetDisk == "" {
			common.Error("Could not detect disk")
			fmt.Println("Please specify: juniper-host bootstrap --disk=/dev/sdX")
			os.Exit(1)
		}
	}

	// Verify disk exists
	if !common.BlockDeviceExists(targetDisk) {
		common.Error(fmt.Sprintf("Disk not found: %s", targetDisk))
		os.Exit(1)
	}

	// Validate disk path format
	if !common.IsValidDiskPath(targetDisk) {
		common.Error(fmt.Sprintf("Invalid disk path format: %s", targetDisk))
		fmt.Println("Expected format: /dev/vda, /dev/sda, /dev/nvme0n1, etc.")
		os.Exit(1)
	}

	common.Header("Juniper Bible - NixOS Bootstrap")
	fmt.Printf("Disk: %s\n\n", targetDisk)

	// Confirm
	common.Warning(fmt.Sprintf("This will ERASE %s", targetDisk))
	if !*yes {
		if !common.Confirm("Continue?", false) {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
	}

	// Get partition paths: bios_grub (1), ESP (2), root (3)
	_, espPart, rootPart := common.GetPartitions(targetDisk)

	// Partition
	common.Info("Partitioning disk...")
	if err := partition(targetDisk); err != nil {
		common.Error(fmt.Sprintf("Partitioning failed: %v", err))
		os.Exit(1)
	}
	time.Sleep(2 * time.Second)

	// Format (only ESP and root - bios_grub partition is not formatted)
	common.Info("Formatting partitions...")
	if err := format(espPart, rootPart); err != nil {
		common.Error(fmt.Sprintf("Formatting failed: %v", err))
		os.Exit(1)
	}

	// Wait for udev
	common.Info("Waiting for disk labels...")
	common.RunQuiet("udevadm", "settle")
	time.Sleep(2 * time.Second)

	// Mount
	common.Info("Mounting filesystems...")
	if err := mount(espPart, rootPart); err != nil {
		common.Error(fmt.Sprintf("Mount failed: %v", err))
		os.Exit(1)
	}

	// Generate hardware config
	common.Info("Generating hardware configuration...")
	if err := common.Run("nixos-generate-config", "--root", "/mnt"); err != nil {
		common.Error(fmt.Sprintf("Failed to generate hardware config: %v", err))
		os.Exit(1)
	}

	// Download configuration
	common.Info("Downloading configuration...")
	configURL := common.RepoBase + "/configuration.nix"
	if err := common.DownloadFile(configURL, "/mnt/etc/nixos/configuration.nix"); err != nil {
		common.Error(fmt.Sprintf("Failed to download configuration: %v", err))
		os.Exit(1)
	}

	// Inject boot device into configuration
	common.Info("Configuring bootloader for " + targetDisk + "...")
	if err := injectBootDevice(targetDisk); err != nil {
		common.Warning(fmt.Sprintf("Failed to configure bootloader: %v", err))
	} else {
		common.Success("Bootloader configured for " + targetDisk)
	}

	// Get SSH key
	// With --enthusiastic-yes, still prompt for SSH key if not provided (safety first)
	key := *sshKey
	if key == "" {
		fmt.Println()
		key = common.Prompt("Enter your SSH public key (ssh-ed25519 or ssh-rsa)", "")
	}

	// Inject SSH key
	if key != "" && common.IsValidSSHKey(key) {
		if err := injectSSHKey(key); err != nil {
			common.Error(fmt.Sprintf("CRITICAL: Failed to inject SSH key: %v", err))
			fmt.Println("\nWithout an SSH key, you will be LOCKED OUT of your server!")
			fmt.Println("You must fix this issue before proceeding.")
			os.Exit(1)
		} else {
			common.Success("SSH key configured")
		}
	}

	// Install NixOS
	fmt.Println()
	common.Info("Installing NixOS...")
	common.Warning("This takes 10-30 minutes on VPS (downloading packages from cache.nixos.org)")
	common.Info("Progress dots will appear every 5 seconds. Do NOT interrupt.")
	fmt.Println()
	if err := common.RunWithProgress("nixos-install", "--no-root-passwd"); err != nil {
		common.Error(fmt.Sprintf("Installation failed: %v", err))
		os.Exit(1)
	}

	fmt.Println()
	common.Header("Installation complete!")
	fmt.Println("Rebooting in 5 seconds... (Ctrl+C to cancel)")
	time.Sleep(5 * time.Second)
	common.Run("reboot")
}

func partition(disk string) error {
	// Partition layout for hybrid BIOS/UEFI boot with GPT:
	// 1. BIOS Boot Partition (1MB) - required for GRUB on GPT+BIOS
	// 2. EFI System Partition (512MB) - for UEFI boot
	// 3. Root partition (rest of disk)
	cmds := [][]string{
		{"parted", disk, "--", "mklabel", "gpt"},
		{"parted", disk, "--", "mkpart", "bios_grub", "1MB", "2MB"},
		{"parted", disk, "--", "set", "1", "bios_grub", "on"},
		{"parted", disk, "--", "mkpart", "ESP", "fat32", "2MB", "514MB"},
		{"parted", disk, "--", "set", "2", "esp", "on"},
		{"parted", disk, "--", "mkpart", "primary", "514MB", "100%"},
	}
	for _, cmd := range cmds {
		if err := common.Run(cmd[0], cmd[1:]...); err != nil {
			return err
		}
	}
	// Sync partition table to kernel
	common.RunQuiet("partprobe", disk)
	return nil
}

func format(espPart, rootPart string) error {
	// Format ESP as FAT32
	if err := common.Run("mkfs.fat", "-F", "32", "-n", "boot", espPart); err != nil {
		return err
	}
	// Format root as ext4
	return common.Run("mkfs.ext4", "-F", "-L", "nixos", rootPart)
}

func mount(espPart, rootPart string) error {
	// Mount root partition first
	if err := common.Run("mount", rootPart, "/mnt"); err != nil {
		return err
	}
	// Create and mount boot directory
	if err := os.MkdirAll("/mnt/boot", 0755); err != nil {
		return err
	}
	return common.Run("mount", espPart, "/mnt/boot")
}

func injectSSHKey(key string) error {
	configPath := "/mnt/etc/nixos/configuration.nix"
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	content := string(data)
	// Escape backslashes first, then quotes (order matters)
	escapedKey := strings.ReplaceAll(key, `\`, `\\`)
	escapedKey = strings.ReplaceAll(escapedKey, `"`, `\"`)

	old := `# "ssh-ed25519 AAAA... your-key-here"`
	new := fmt.Sprintf(`"%s"`, escapedKey)
	content = replaceFirst(content, old, new)

	return os.WriteFile(configPath, []byte(content), 0600)
}

func injectBootDevice(disk string) error {
	configPath := "/mnt/etc/nixos/configuration.nix"
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	content := string(data)
	// Escape backslashes first, then quotes (order matters for Nix strings)
	escapedDisk := strings.ReplaceAll(disk, `\`, `\\`)
	escapedDisk = strings.ReplaceAll(escapedDisk, `"`, `\"`)
	escapedDisk = strings.ReplaceAll(escapedDisk, `$`, `\$`)

	// Replace the default /dev/vda with the actual disk
	content = strings.Replace(content, `device = "/dev/vda";`, fmt.Sprintf(`device = "%s";`, escapedDisk), 1)

	return os.WriteFile(configPath, []byte(content), 0600)
}

func replaceFirst(s, old, new string) string {
	i := indexOf(s, old)
	if i < 0 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func indexOf(s, substr string) int {
	if substr == "" {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
