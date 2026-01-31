package bootstrap

import (
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/common"
)

// Run executes the bootstrap command
func Run(args []string) {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	disk := fs.String("disk", "", "Target disk (auto-detect if not specified)")
	sshKey := fs.String("ssh-key", "", "SSH public key")
	yes := fs.Bool("yes", false, "Skip confirmation prompts")
	fs.Parse(args)

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

	part1, part2 := common.GetPartitions(targetDisk)

	// Partition
	common.Info("Partitioning disk...")
	if err := partition(targetDisk); err != nil {
		common.Error(fmt.Sprintf("Partitioning failed: %v", err))
		os.Exit(1)
	}
	time.Sleep(2 * time.Second)

	// Format
	common.Info("Formatting partitions...")
	if err := format(part1, part2); err != nil {
		common.Error(fmt.Sprintf("Formatting failed: %v", err))
		os.Exit(1)
	}

	// Wait for udev
	common.Info("Waiting for disk labels...")
	common.RunQuiet("udevadm", "settle")
	time.Sleep(2 * time.Second)

	// Mount
	common.Info("Mounting filesystems...")
	if err := mount(part1, part2); err != nil {
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

	// Get SSH key
	key := *sshKey
	if key == "" && !*yes {
		fmt.Println()
		key = common.Prompt("Enter your SSH public key (ssh-ed25519 or ssh-rsa)", "")
	}

	// Inject SSH key
	if key != "" && common.IsValidSSHKey(key) {
		if err := injectSSHKey(key); err != nil {
			common.Warning(fmt.Sprintf("Failed to inject SSH key: %v", err))
		} else {
			common.Success("SSH key configured")
		}
	}

	// Install NixOS
	common.Info("Installing NixOS (this takes a few minutes)...")
	if err := common.Run("nixos-install", "--no-root-passwd"); err != nil {
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
	cmds := [][]string{
		{"parted", disk, "--", "mklabel", "gpt"},
		{"parted", disk, "--", "mkpart", "ESP", "fat32", "1MB", "512MB"},
		{"parted", disk, "--", "set", "1", "esp", "on"},
		{"parted", disk, "--", "mkpart", "primary", "512MB", "100%"},
	}
	for _, cmd := range cmds {
		if err := common.Run(cmd[0], cmd[1:]...); err != nil {
			return err
		}
	}
	return nil
}

func format(part1, part2 string) error {
	if err := common.Run("mkfs.fat", "-F", "32", "-n", "boot", part1); err != nil {
		return err
	}
	return common.Run("mkfs.ext4", "-F", "-L", "nixos", part2)
}

func mount(part1, part2 string) error {
	if err := common.Run("mount", part2, "/mnt"); err != nil {
		return err
	}
	if err := os.MkdirAll("/mnt/boot", 0755); err != nil {
		return err
	}
	return common.Run("mount", part1, "/mnt/boot")
}

func injectSSHKey(key string) error {
	configPath := "/mnt/etc/nixos/configuration.nix"
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	content := string(data)
	old := `# "ssh-ed25519 AAAA... your-key-here"`
	new := fmt.Sprintf(`"%s"`, key)
	content = replaceFirst(content, old, new)

	return os.WriteFile(configPath, []byte(content), 0644)
}

func replaceFirst(s, old, new string) string {
	i := indexOf(s, old)
	if i < 0 {
		return s
	}
	return s[:i] + new + s[i+len(old):]
}

func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
