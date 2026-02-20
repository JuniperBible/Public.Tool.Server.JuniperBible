package bootstrap

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/common"
)

// bootstrapFlags holds all command line flags for bootstrap
type bootstrapFlags struct {
	disk            string
	sshKey          string
	sshKeyFile      string
	yes             bool
	enthusiasticYes bool
}

// parseFlags parses command line arguments and returns bootstrapFlags
func parseFlags(args []string) bootstrapFlags {
	fs := flag.NewFlagSet("bootstrap", flag.ExitOnError)
	disk := fs.String("disk", "", "Target disk (auto-detect if not specified)")
	sshKey := fs.String("ssh-key", "", "SSH public key")
	sshKeyFile := fs.String("ssh-key-file", "", "Path to SSH public key file")
	yes := fs.Bool("yes", false, "Skip confirmation prompts")
	enthusiasticYes := fs.Bool("enthusiastic-yes", false, "Auto-detect everything, only prompt for SSH key if not provided")
	if err := fs.Parse(args); err != nil {
		common.Error(fmt.Sprintf("Failed to parse arguments: %v", err))
		os.Exit(1)
	}

	flags := bootstrapFlags{
		disk:            *disk,
		sshKey:          *sshKey,
		sshKeyFile:      *sshKeyFile,
		yes:             *yes,
		enthusiasticYes: *enthusiasticYes,
	}

	// --enthusiastic-yes implies --yes for disk confirmation
	if flags.enthusiasticYes {
		flags.yes = true
	}

	return flags
}

// findFirstValidKey finds the first non-comment, non-empty line
func findFirstValidKey(content string) (string, error) {
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, "#") {
			return line, nil
		}
	}
	return "", fmt.Errorf("no valid SSH key found in file")
}

// readSSHKeyFromFile reads and validates an SSH key from a file
func readSSHKeyFromFile(path string) (string, error) {
	if strings.Contains(path, "..") {
		return "", fmt.Errorf("SSH key file path cannot contain '..'")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read SSH key file: %w", err)
	}
	keyStr := strings.TrimSpace(string(data))
	if keyStr == "" {
		return "", fmt.Errorf("SSH key file is empty")
	}
	return findFirstValidKey(keyStr)
}

// validateAndDetectDisk validates disk path or auto-detects it
func validateAndDetectDisk(diskFlag string) string {
	targetDisk := diskFlag
	if targetDisk == "" {
		targetDisk = common.DetectDisk()
		if targetDisk == "" {
			common.Error("Could not detect disk")
			fmt.Println("Please specify: juniper-host bootstrap --disk=/dev/sdX")
			os.Exit(1)
		}
	}

	if !common.BlockDeviceExists(targetDisk) {
		common.Error(fmt.Sprintf("Disk not found: %s", targetDisk))
		os.Exit(1)
	}

	if !common.IsValidDiskPath(targetDisk) {
		common.Error(fmt.Sprintf("Invalid disk path format: %s", targetDisk))
		fmt.Println("Expected format: /dev/vda, /dev/sda, /dev/nvme0n1, etc.")
		os.Exit(1)
	}

	return targetDisk
}

// promptForSSHKey prompts user for SSH key if not provided
func promptForSSHKey(existingKey string) string {
	if existingKey != "" {
		return existingKey
	}
	fmt.Println()
	const maxKeyRetries = 5
	for attempt := 0; attempt < maxKeyRetries; attempt++ {
		key := common.Prompt("Enter your SSH public key (ssh-ed25519 or ssh-rsa)", "")
		if key != "" {
			return key
		}
		if attempt < maxKeyRetries-1 {
			common.Warning("No SSH key entered. You may be locked out without one.")
		}
	}
	common.Warning("No SSH key provided. Continuing without SSH key.")
	return ""
}

// configureSSHKey validates and injects the SSH key into configuration
func configureSSHKey(key string) {
	if key == "" {
		return
	}
	if !common.IsValidSSHKey(key) {
		common.Warning("SSH key failed validation (invalid format). Continuing without SSH key.")
		common.Warning("You may be locked out of the server!")
		return
	}
	if err := injectSSHKey(key); err != nil {
		common.Error(fmt.Sprintf("CRITICAL: Failed to inject SSH key: %v", err))
		fmt.Println("\nWithout an SSH key, you will be LOCKED OUT of your server!")
		fmt.Println("You must fix this issue before proceeding.")
		os.Exit(1)
	}
	common.Success("SSH key configured for deploy and root users")
}

// prepareFilesystems partitions, formats, and mounts the disk
func prepareFilesystems(targetDisk string) {
	_, espPart, rootPart := common.GetPartitions(targetDisk)

	common.Info("Partitioning disk...")
	if err := partition(targetDisk); err != nil {
		common.Error(fmt.Sprintf("Partitioning failed: %v", err))
		os.Exit(1)
	}
	time.Sleep(2 * time.Second)

	common.Info("Formatting partitions...")
	if err := format(espPart, rootPart); err != nil {
		common.Error(fmt.Sprintf("Formatting failed: %v", err))
		os.Exit(1)
	}

	common.Info("Waiting for disk labels...")
	if err := common.RunQuiet("udevadm", "settle"); err != nil {
		common.Warning(fmt.Sprintf("udevadm settle returned error: %v (continuing anyway)", err))
	}
	time.Sleep(2 * time.Second)

	common.Info("Mounting filesystems...")
	if err := mount(espPart, rootPart); err != nil {
		common.Error(fmt.Sprintf("Mount failed: %v", err))
		os.Exit(1)
	}
}

// downloadAndConfigureNixOS downloads config and generates hardware config
func downloadAndConfigureNixOS(targetDisk string) {
	common.Info("Generating hardware configuration...")
	if err := common.Run("nixos-generate-config", "--root", "/mnt"); err != nil {
		common.Error(fmt.Sprintf("Failed to generate hardware config: %v", err))
		os.Exit(1)
	}

	common.Info("Downloading configuration...")
	configURL := common.RepoBase + "/configuration.nix"
	if err := common.DownloadFile(configURL, "/mnt/etc/nixos/configuration.nix"); err != nil {
		common.Error(fmt.Sprintf("Failed to download configuration: %v", err))
		os.Exit(1)
	}

	common.Info("Configuring bootloader for " + targetDisk + "...")
	if err := injectBootDevice(targetDisk); err != nil {
		common.Warning(fmt.Sprintf("Failed to configure bootloader: %v", err))
	} else {
		common.Success("Bootloader configured for " + targetDisk)
	}
}

// installNixOS runs the NixOS installation
func installNixOS() {
	fmt.Println()
	common.Info("Installing NixOS...")
	common.Warning("This takes 10-30 minutes on VPS (downloading packages from cache.nixos.org)")
	common.Info("Progress dots will appear every 5 seconds. Do NOT interrupt.")
	fmt.Println()
	if err := common.RunWithProgress("nixos-install", "--no-root-passwd"); err != nil {
		common.Error(fmt.Sprintf("Installation failed: %v", err))
		os.Exit(1)
	}
}

// resolveSSHKey gets SSH key from flags or file
func resolveSSHKey(flags bootstrapFlags) string {
	if flags.sshKeyFile != "" && flags.sshKey == "" {
		key, err := readSSHKeyFromFile(flags.sshKeyFile)
		if err != nil {
			common.Error(err.Error())
			os.Exit(1)
		}
		return key
	}
	return flags.sshKey
}

// confirmDiskErase prompts user to confirm disk erasure
func confirmDiskErase(targetDisk string, yes bool) {
	common.Warning(fmt.Sprintf("This will ERASE %s", targetDisk))
	if !yes && !common.Confirm("Continue?", false) {
		fmt.Println("Aborted.")
		os.Exit(0)
	}
}

// completeInstallation finishes installation and reboots
func completeInstallation() {
	fmt.Println()
	common.Header("Installation complete!")
	fmt.Println("Rebooting in 5 seconds... (Ctrl+C to cancel)")
	time.Sleep(5 * time.Second)
	if err := common.Run("reboot"); err != nil {
		common.Warning(fmt.Sprintf("Reboot command failed: %v", err))
		fmt.Println("Please reboot manually to complete installation.")
	}
}

// Run executes the bootstrap command
func Run(args []string) {
	flags := parseFlags(args)
	sshKey := resolveSSHKey(flags)

	if !common.IsRoot() {
		common.Error("Must be run as root")
		fmt.Println("Usage: sudo juniper-host bootstrap")
		os.Exit(1)
	}

	targetDisk := validateAndDetectDisk(flags.disk)

	common.Header("Juniper Bible - NixOS Bootstrap")
	fmt.Printf("Disk: %s\n\n", targetDisk)

	confirmDiskErase(targetDisk, flags.yes)

	prepareFilesystems(targetDisk)
	downloadAndConfigureNixOS(targetDisk)

	sshKey = promptForSSHKey(sshKey)
	configureSSHKey(sshKey)

	installNixOS()
	completeInstallation()
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
	originalContent := content

	// Escape for Nix string literals: backslashes, quotes, and $ (interpolation)
	escapedKey := strings.ReplaceAll(key, `\`, `\\`)
	escapedKey = strings.ReplaceAll(escapedKey, `"`, `\"`)
	escapedKey = strings.ReplaceAll(escapedKey, `$`, `\$`)

	// Replace both deploy and root user SSH key placeholders
	old := `# "ssh-ed25519 AAAA... your-key-here"`
	new := fmt.Sprintf(`"%s"`, escapedKey)
	// Replace all occurrences (deploy and root users)
	content = strings.ReplaceAll(content, old, new)

	// Verify replacement occurred
	if content == originalContent {
		return fmt.Errorf("SSH key placeholder not found in configuration")
	}

	return os.WriteFile(configPath, []byte(content), 0600)
}

func injectBootDevice(disk string) error {
	configPath := "/mnt/etc/nixos/configuration.nix"
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	content := string(data)
	originalContent := content

	// Escape backslashes first, then quotes (order matters for Nix strings)
	escapedDisk := strings.ReplaceAll(disk, `\`, `\\`)
	escapedDisk = strings.ReplaceAll(escapedDisk, `"`, `\"`)
	escapedDisk = strings.ReplaceAll(escapedDisk, `$`, `\$`)

	// Replace the default /dev/vda with the actual disk
	content = strings.Replace(content, `device = "/dev/vda";`, fmt.Sprintf(`device = "%s";`, escapedDisk), 1)

	// Verify replacement occurred (only warn, don't fail - disk might already be correct)
	if content == originalContent {
		return fmt.Errorf("boot device placeholder '/dev/vda' not found")
	}

	return os.WriteFile(configPath, []byte(content), 0600)
}

