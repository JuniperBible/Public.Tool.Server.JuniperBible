package upgrade

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/JuniperBible/juniper-server/internal/common"
)

const (
	configURL = common.RepoBase + "/configuration.nix"
)

// Run executes the upgrade command
func Run(args []string) {
	fs := flag.NewFlagSet("upgrade", flag.ExitOnError)
	host := fs.String("host", "", "Remote host (e.g., root@server or root@192.168.1.1)")
	sshKey := fs.String("i", "", "SSH identity file (optional)")
	yes := fs.Bool("yes", false, "Skip confirmation prompts")
	configOnly := fs.Bool("config-only", false, "Only update configuration, don't rebuild")

	if err := fs.Parse(args); err != nil {
		common.Error(fmt.Sprintf("Failed to parse arguments: %v", err))
		os.Exit(1)
	}

	// Check if host is provided
	if *host == "" {
		// Check if we're running locally on a NixOS system
		if common.FileExists("/etc/nixos/configuration.nix") {
			runLocalUpgrade(*yes, *configOnly)
			return
		}
		common.Error("No host specified and not running on NixOS")
		fmt.Println()
		fmt.Println("Usage: juniper-host upgrade --host=root@server")
		fmt.Println("       juniper-host upgrade  (when running on the server itself)")
		os.Exit(1)
	}

	runRemoteUpgrade(*host, *sshKey, *yes, *configOnly)
}

// backupAndDownloadConfig backs up current config and downloads new one
func backupAndDownloadConfig() (sshKeys []string) {
	common.Info("Backing up current configuration...")
	if err := common.Run("cp", "/etc/nixos/configuration.nix", "/etc/nixos/configuration.nix.pre-upgrade"); err != nil {
		common.Error(fmt.Sprintf("Failed to backup config: %v", err))
		os.Exit(1)
	}

	common.Info("Extracting SSH keys from current configuration...")
	sshKeys = extractSSHKeys("/etc/nixos/configuration.nix")

	common.Info("Downloading latest configuration...")
	if err := common.DownloadFile(configURL, "/etc/nixos/configuration.nix.new"); err != nil {
		common.Error(fmt.Sprintf("Failed to download configuration: %v", err))
		os.Exit(1)
	}

	if len(sshKeys) > 0 {
		common.Info(fmt.Sprintf("Injecting %d SSH key(s) into new configuration...", len(sshKeys)))
		if err := injectSSHKeys("/etc/nixos/configuration.nix.new", sshKeys); err != nil {
			common.Error(fmt.Sprintf("Failed to inject SSH keys: %v", err))
			os.Exit(1)
		}
	}
	return
}

// showDiffAndConfirm shows diff and asks for confirmation
func showDiffAndConfirm(yes bool) {
	fmt.Println()
	common.Info("Configuration changes:")
	diffCmd := exec.Command("diff", "-u", "/etc/nixos/configuration.nix.pre-upgrade", "/etc/nixos/configuration.nix.new")
	diffCmd.Stdout = os.Stdout
	diffCmd.Stderr = os.Stderr
	diffCmd.Run() // Ignore error - diff returns non-zero if files differ

	if !yes {
		fmt.Println()
		if !common.Confirm("Apply this upgrade?", true) {
			common.Info("Upgrade cancelled")
			os.Remove("/etc/nixos/configuration.nix.new")
			os.Exit(0)
		}
	}
}

// applyLocalConfig applies new config and optionally rebuilds NixOS
func applyLocalConfig(configOnly bool) {
	common.Info("Applying new configuration...")
	if err := os.Rename("/etc/nixos/configuration.nix.new", "/etc/nixos/configuration.nix"); err != nil {
		common.Error(fmt.Sprintf("Failed to apply configuration: %v", err))
		os.Exit(1)
	}

	if configOnly {
		common.Success("Configuration updated (rebuild skipped)")
		fmt.Println()
		fmt.Println("Run 'nixos-rebuild switch' to apply changes")
		return
	}

	fmt.Println()
	common.Info("Rebuilding NixOS...")
	if err := common.Run("nixos-rebuild", "switch"); err != nil {
		common.Error("NixOS rebuild failed. Restoring backup...")
		if restoreErr := os.Rename("/etc/nixos/configuration.nix.pre-upgrade", "/etc/nixos/configuration.nix"); restoreErr != nil {
			common.Error(fmt.Sprintf("Failed to restore backup: %v", restoreErr))
		} else {
			common.Success("Backup restored")
		}
		os.Exit(1)
	}

	fmt.Println()
	common.Success("Upgrade complete!")
}

func runLocalUpgrade(yes, configOnly bool) {
	common.Header("Juniper Bible - Local Upgrade")
	common.Info("Checking for updates...")

	backupAndDownloadConfig()
	showDiffAndConfirm(yes)
	applyLocalConfig(configOnly)
}

// buildSSHArgs constructs SSH command arguments
func buildSSHArgs(sshKeyPath string) []string {
	sshArgs := []string{}
	if sshKeyPath != "" {
		sshArgs = append(sshArgs, "-i", sshKeyPath)
	}
	sshArgs = append(sshArgs, "-o", "StrictHostKeyChecking=accept-new")
	return sshArgs
}

// testSSHConnection tests SSH connectivity to the host
func testSSHConnection(sshArgs []string, host string) {
	common.Info("Testing SSH connection...")
	testCmd := exec.Command("ssh", append(sshArgs, host, "echo 'Connected'")...)
	testCmd.Stderr = os.Stderr
	if err := testCmd.Run(); err != nil {
		common.Error(fmt.Sprintf("SSH connection failed: %v", err))
		os.Exit(1)
	}
	common.Success("SSH connection OK")
}

// getRebuildScript returns the rebuild portion of the upgrade script
func getRebuildScript(configOnly bool) string {
	if configOnly {
		return `echo "==> Rebuild skipped (--config-only)"`
	}
	return `echo "==> Rebuilding NixOS..."
if ! nixos-rebuild switch; then
  echo "==> Rebuild failed, restoring backup..."
  mv "$BACKUP" "$CONFIG"
  exit 1
fi`
}

// confirmRemoteUpgrade shows what will happen and asks for confirmation
func confirmRemoteUpgrade(yes, configOnly bool) {
	if yes {
		return
	}
	fmt.Println()
	fmt.Println("This will:")
	fmt.Println("  1. Backup current configuration")
	fmt.Println("  2. Download latest configuration from GitHub")
	fmt.Println("  3. Preserve existing SSH keys")
	if !configOnly {
		fmt.Println("  4. Rebuild NixOS with new configuration")
	}
	fmt.Println()
	if !common.Confirm("Proceed with upgrade?", true) {
		common.Info("Upgrade cancelled")
		os.Exit(0)
	}
}

func runRemoteUpgrade(host, sshKeyPath string, yes, configOnly bool) {
	common.Header("Juniper Bible - Remote Upgrade")
	common.Info(fmt.Sprintf("Target: %s", host))

	sshArgs := buildSSHArgs(sshKeyPath)
	testSSHConnection(sshArgs, host)

	upgradeScript := fmt.Sprintf(`
set -euo pipefail

CONFIG="/etc/nixos/configuration.nix"
CONFIG_URL="%s"
BACKUP="$CONFIG.pre-upgrade"

echo "==> Backing up current configuration..."
cp "$CONFIG" "$BACKUP"

echo "==> Extracting SSH keys..."
DEPLOY_KEYS=$(grep -A20 'users.users.deploy.openssh.authorizedKeys.keys' "$CONFIG" | grep -oP '^\s*"(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp[0-9]+)\s+[A-Za-z0-9+/]+=*(\s+[^"]*)?(?=")' | head -20 || true)
ROOT_KEYS=$(grep -A20 'users.users.root.openssh.authorizedKeys.keys' "$CONFIG" | grep -oP '^\s*"(ssh-ed25519|ssh-rsa|ecdsa-sha2-nistp[0-9]+)\s+[A-Za-z0-9+/]+=*(\s+[^"]*)?(?=")' | head -20 || true)

echo "==> Downloading latest configuration..."
curl -fsSL "$CONFIG_URL" -o "$CONFIG.new"

echo "==> Injecting SSH keys..."
if [ -n "$DEPLOY_KEYS" ]; then
  sed -i '/users.users.deploy.openssh.authorizedKeys.keys = \[/,/\];/{
    /# "ssh-ed25519 AAAA... your-key-here"/d
  }' "$CONFIG.new"
  while IFS= read -r key; do
    [ -z "$key" ] && continue
    key_line=$(echo "$key" | sed 's/^\s*/    /')'"'
    sed -i "/users.users.deploy.openssh.authorizedKeys.keys = \[/a\\$key_line" "$CONFIG.new"
  done <<< "$DEPLOY_KEYS"
fi

if [ -n "$ROOT_KEYS" ]; then
  sed -i '/users.users.root.openssh.authorizedKeys.keys = \[/,/\];/{
    /# "ssh-ed25519 AAAA... your-key-here"/d
  }' "$CONFIG.new"
  while IFS= read -r key; do
    [ -z "$key" ] && continue
    key_line=$(echo "$key" | sed 's/^\s*/    /')'"'
    sed -i "/users.users.root.openssh.authorizedKeys.keys = \[/a\\$key_line" "$CONFIG.new"
  done <<< "$ROOT_KEYS"
fi

echo "==> Showing diff..."
diff -u "$BACKUP" "$CONFIG.new" || true

echo ""
echo "==> Applying new configuration..."
mv "$CONFIG.new" "$CONFIG"

%s

echo ""
echo "==> Upgrade complete!"
`, configURL, getRebuildScript(configOnly))

	confirmRemoteUpgrade(yes, configOnly)

	fmt.Println()
	common.Info("Running upgrade on remote host...")
	fmt.Println()

	sshCmd := exec.Command("ssh", append(sshArgs, host, "bash", "-c", upgradeScript)...)
	sshCmd.Stdout = os.Stdout
	sshCmd.Stderr = os.Stderr
	if err := sshCmd.Run(); err != nil {
		common.Error(fmt.Sprintf("Remote upgrade failed: %v", err))
		os.Exit(1)
	}

	fmt.Println()
	common.Success("Remote upgrade complete!")
}

// isSSHKeyLine checks if a line contains an SSH key
func isSSHKeyLine(line string) bool {
	line = strings.TrimSpace(line)
	return strings.HasPrefix(line, "\"ssh-") || strings.HasPrefix(line, "\"ecdsa-")
}

// extractKeyFromLine extracts the SSH key from a quoted line
func extractKeyFromLine(line string) string {
	return strings.Trim(strings.TrimSpace(line), "\"")
}

// containsKey checks if a key already exists in the slice
func containsKey(keys []string, key string) bool {
	for _, k := range keys {
		if k == key {
			return true
		}
	}
	return false
}

// processSSHKeyLine processes a single line and adds key to slice if valid
func processSSHKeyLine(line string, keys *[]string) {
	if !isSSHKeyLine(line) {
		return
	}
	key := extractKeyFromLine(line)
	if key != "" && !containsKey(*keys, key) {
		*keys = append(*keys, key)
	}
}

// parseLine determines if we enter/exit keys section and processes key lines
func parseLine(line string, inKeysSection bool, keys *[]string) bool {
	if strings.Contains(line, "authorizedKeys.keys = [") {
		return true
	}
	if strings.Contains(line, "];") {
		return false
	}
	if inKeysSection {
		processSSHKeyLine(line, keys)
	}
	return inKeysSection
}

func extractSSHKeys(configPath string) []string {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var keys []string
	inKeysSection := false
	for _, line := range strings.Split(string(data), "\n") {
		inKeysSection = parseLine(line, inKeysSection, &keys)
	}
	return keys
}

func injectSSHKeys(configPath string, keys []string) error {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return err
	}

	content := string(data)

	// Build the keys string
	var keysStr strings.Builder
	for _, key := range keys {
		// Escape for Nix
		escapedKey := strings.ReplaceAll(key, `\`, `\\`)
		escapedKey = strings.ReplaceAll(escapedKey, `"`, `\"`)
		escapedKey = strings.ReplaceAll(escapedKey, `$`, `\$`)
		keysStr.WriteString(fmt.Sprintf("    \"%s\"\n", escapedKey))
	}

	// Replace placeholder in deploy user keys
	content = strings.Replace(content,
		"users.users.deploy.openssh.authorizedKeys.keys = [\n    # \"ssh-ed25519 AAAA... your-key-here\"\n  ];",
		fmt.Sprintf("users.users.deploy.openssh.authorizedKeys.keys = [\n%s  ];", keysStr.String()),
		1)

	// Replace placeholder in root user keys
	content = strings.Replace(content,
		"users.users.root.openssh.authorizedKeys.keys = [\n    # \"ssh-ed25519 AAAA... your-key-here\"\n  ];",
		fmt.Sprintf("users.users.root.openssh.authorizedKeys.keys = [\n%s  ];", keysStr.String()),
		1)

	return os.WriteFile(configPath, []byte(content), 0600)
}
