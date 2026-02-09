package wizard

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/JuniperBible/Website.Server.JuniperBible.org/internal/common"
)

const (
	nixosConfig   = "/etc/nixos/configuration.nix"
	setupDoneFlag = "/etc/juniper-setup-complete"
)

// Run executes the setup wizard
func Run(args []string) {
	// Check if already completed
	if common.FileExists(setupDoneFlag) {
		return
	}

	// Gather system info
	hostname := common.GetHostname()
	ip := common.GetIP()
	osVersion := common.GetOSVersion()
	kernel := common.GetKernel()

	// Welcome screen
	common.ClearScreen()
	common.Banner(hostname, ip, osVersion, kernel)
	common.WaitForEnter("Press Enter to continue...")

	// Step 1: Hostname
	common.Step(1, 4, "Hostname")
	fmt.Printf("Current hostname: %s%s%s\n\n", common.Cyan, hostname, common.Reset)
	var newHostname string
	const maxRetries = 5
	for attempts := 0; attempts < maxRetries; attempts++ {
		newHostname = common.Prompt("Enter new hostname (or press Enter to keep current)", hostname)
		if common.IsValidHostname(newHostname) {
			break
		}
		common.Error("Invalid hostname. Use alphanumerics and hyphens only (1-63 chars).")
		if attempts == maxRetries-1 {
			common.Error("Too many invalid attempts. Setup cancelled.")
			os.Exit(1)
		}
	}

	// Step 2: Domain
	common.Step(2, 4, "Domain")
	fmt.Println("Enter your domain for HTTPS (Caddy will auto-provision certificates).")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  - juniperbible.org")
	fmt.Println("  - bible.example.com")
	fmt.Println("  - localhost (for testing, no HTTPS)")
	fmt.Println()
	var domain string
	for attempts := 0; attempts < maxRetries; attempts++ {
		domain = common.Prompt("Domain", "localhost")
		if common.IsValidDomain(domain) {
			break
		}
		common.Error("Invalid domain. Use alphanumerics, hyphens, and dots only.")
		if attempts == maxRetries-1 {
			common.Error("Too many invalid attempts. Setup cancelled.")
			os.Exit(1)
		}
	}

	// Step 3: SSH Keys
	common.Step(3, 4, "SSH Keys")
	fmt.Println("Add SSH public keys for the 'deploy' user.")
	fmt.Println("Paste one key per line. Enter empty line when done.")
	fmt.Println()
	fmt.Printf("%sWARNING: If you don't add a key, you may be locked out!%s\n\n", common.Yellow, common.Reset)

	const maxSSHKeys = 50
	var sshKeys []string
	for {
		if len(sshKeys) >= maxSSHKeys {
			common.Warning(fmt.Sprintf("Maximum of %d SSH keys reached.", maxSSHKeys))
			break
		}
		key := common.Prompt("SSH key (or Enter to finish)", "")
		if key == "" {
			break
		}
		if common.IsValidSSHKey(key) {
			sshKeys = append(sshKeys, key)
			common.Success("Key added")
		} else {
			common.Error("Invalid key format. Keys should start with ssh-ed25519, ssh-rsa, or ecdsa-*")
		}
	}

	if len(sshKeys) == 0 {
		fmt.Println()
		common.Error("No SSH keys added! You may be locked out after reboot.")
		if !common.Confirm("Continue anyway?", false) {
			fmt.Println("Setup cancelled. Run 'juniper-host wizard' to try again.")
			os.Exit(1)
		}
	}

	// Step 4: Deploy
	common.Step(4, 4, "Deploy Site")
	fmt.Println("Would you like to deploy Juniper Bible now?")
	fmt.Println()
	deployNow := common.Confirm("Deploy site?", true)

	// Summary
	common.ClearScreen()
	fmt.Printf("%sConfiguration Summary%s\n\n", common.Bold, common.Reset)
	fmt.Printf("  Hostname: %s%s%s\n", common.Cyan, newHostname, common.Reset)
	fmt.Printf("  Domain:   %s%s%s\n", common.Cyan, domain, common.Reset)
	fmt.Printf("  SSH Keys: %s%d key(s)%s\n", common.Cyan, len(sshKeys), common.Reset)
	deployStr := "No"
	if deployNow {
		deployStr = "Yes"
	}
	fmt.Printf("  Deploy:   %s%s%s\n", common.Cyan, deployStr, common.Reset)
	fmt.Println()

	if !common.Confirm("Apply this configuration?", true) {
		fmt.Println("Setup cancelled. Run 'juniper-host wizard' to try again.")
		os.Exit(1)
	}

	// Apply configuration
	fmt.Println()
	fmt.Printf("%sApplying configuration...%s\n\n", common.Bold, common.Reset)

	// Backup config (fatal if fails - we need to be able to restore)
	if err := copyFile(nixosConfig, nixosConfig+".backup"); err != nil {
		common.Error(fmt.Sprintf("Failed to backup config: %v", err))
		os.Exit(1)
	}

	// Update configuration
	if err := updateConfig(newHostname, domain, sshKeys); err != nil {
		common.Error(fmt.Sprintf("Failed to update configuration: %v", err))
		os.Exit(1)
	}
	common.Success("Configuration updated")

	// Rebuild NixOS
	fmt.Println()
	fmt.Println("Rebuilding NixOS (this may take a minute)...")
	if err := common.Run("nixos-rebuild", "switch"); err != nil {
		common.Error("NixOS rebuild failed. Check /etc/nixos/configuration.nix")
		fmt.Printf("  Backup saved to %s.backup\n", nixosConfig)
		os.Exit(1)
	}
	common.Success("NixOS rebuilt successfully")

	// Mark setup complete
	if err := os.WriteFile(setupDoneFlag, []byte{}, 0600); err != nil {
		common.Warning(fmt.Sprintf("Failed to create setup flag: %v", err))
	}

	// Deploy if requested
	if deployNow {
		fmt.Println()
		fmt.Println("Deploying Juniper Bible...")
		if err := common.Run("/etc/deploy-juniper.sh"); err != nil {
			common.Warning("Site deployment failed. You can try again with: deploy-juniper")
		} else {
			common.Success("Site deployed successfully")
		}
	}

	// Done
	fmt.Println()
	fmt.Printf("%s%sSetup Complete!%s\n\n", common.Green, common.Bold, common.Reset)
	fmt.Println("Your Juniper Bible server is ready.")
	fmt.Println()
	if domain != "localhost" {
		fmt.Printf("  Website: %shttps://%s%s\n", common.Cyan, domain, common.Reset)
	} else {
		fmt.Printf("  Website: %shttp://%s%s\n", common.Cyan, domain, common.Reset)
	}
	fmt.Printf("  SSH:     %sssh deploy@%s%s\n", common.Cyan, common.GetIP(), common.Reset)
	fmt.Println()
	fmt.Println("Useful commands:")
	fmt.Println("  deploy-juniper              - Update the site")
	fmt.Println("  sudo nixos-rebuild switch   - Apply config changes")
	fmt.Println()
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0600)
}

func updateConfig(hostname, domain string, sshKeys []string) error {
	data, err := os.ReadFile(nixosConfig)
	if err != nil {
		return err
	}
	content := string(data)

	// Update hostname (escape for regex replacement)
	hostnameRe := regexp.MustCompile(`networking\.hostName = "[^"]*"`)
	escapedHostname := escapeNixString(hostname)
	content = hostnameRe.ReplaceAllLiteralString(content, fmt.Sprintf(`networking.hostName = "%s"`, escapedHostname))

	// Update domain (escape for regex replacement)
	domainRe := regexp.MustCompile(`services\.caddy\.virtualHosts\."[^"]*"\.extraConfig`)
	escapedDomain := escapeNixString(domain)
	content = domainRe.ReplaceAllLiteralString(content, fmt.Sprintf(`services.caddy.virtualHosts."%s".extraConfig`, escapedDomain))

	// Update SSH keys
	if len(sshKeys) > 0 {
		var keysNix strings.Builder
		keysNix.WriteString("users.users.deploy.openssh.authorizedKeys.keys = [\n")
		for _, key := range sshKeys {
			escapedKey := escapeNixString(key)
			keysNix.WriteString(fmt.Sprintf("    \"%s\"\n", escapedKey))
		}
		keysNix.WriteString("  ];")

		keysRe := regexp.MustCompile(`users\.users\.deploy\.openssh\.authorizedKeys\.keys = \[[\s\S]*?\];`)
		content = keysRe.ReplaceAllLiteralString(content, keysNix.String())
	}

	return os.WriteFile(nixosConfig, []byte(content), 0600)
}

// escapeNixString escapes special characters for Nix string literals
func escapeNixString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `$`, `\$`)
	return s
}
