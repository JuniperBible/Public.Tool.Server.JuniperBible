package wizard

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/JuniperBible/juniper-server/internal/common"
)

const (
	nixosConfig   = "/etc/nixos/configuration.nix"
	caddyfile     = "/var/lib/caddy/Caddyfile"
	setupDoneFlag = "/etc/juniper-setup-complete"
)

// TLS mode constants
const (
	TLSModeACMEHTTP   = "1"
	TLSModeACMEDNS    = "2"
	TLSModeCustomCert = "3"
	TLSModeHTTPOnly   = "4"
	TLSModeSelfSigned = "5"
)

// wizardConfig holds all collected wizard configuration
type wizardConfig struct {
	hostname   string
	domain     string
	tlsMode    string
	cfAPIToken string
	certPath   string
	keyPath    string
	sshKeys    []string
	deployNow  bool
}

// promptHostname prompts for and validates hostname
func promptHostname(current string) string {
	common.Step(1, 5, "Hostname")
	fmt.Printf("Current hostname: %s%s%s\n\n", common.Cyan, current, common.Reset)
	const maxRetries = 5
	for attempts := 0; attempts < maxRetries; attempts++ {
		hostname := common.Prompt("Enter new hostname (or press Enter to keep current)", current)
		if common.IsValidHostname(hostname) {
			return hostname
		}
		common.Error("Invalid hostname. Use alphanumerics and hyphens only (1-63 chars).")
		if attempts == maxRetries-1 {
			common.Error("Too many invalid attempts. Setup cancelled.")
			os.Exit(1)
		}
	}
	return current
}

// promptDomain prompts for and validates domain
func promptDomain() string {
	common.Step(2, 5, "Domain")
	fmt.Println("Enter your domain (e.g., juniperbible.org)")
	fmt.Println()
	const maxRetries = 5
	for attempts := 0; attempts < maxRetries; attempts++ {
		domain := common.Prompt("Domain", "localhost")
		if common.IsValidDomain(domain) {
			return domain
		}
		common.Error("Invalid domain. Use alphanumerics, hyphens, and dots only.")
		if attempts == maxRetries-1 {
			common.Error("Too many invalid attempts. Setup cancelled.")
			os.Exit(1)
		}
	}
	return "localhost"
}

// promptACMEDNS prompts for Cloudflare API token
func promptACMEDNS() (token string, fallback bool) {
	fmt.Println()
	fmt.Println("Enter your Cloudflare API token (needs Zone:DNS:Edit permission):")
	token = common.Prompt("CF API Token", "")
	if token == "" {
		common.Warning("API token required for DNS-01. Falling back to self-signed.")
		return "", true
	}
	return token, false
}

// promptCustomCert prompts for certificate paths
func promptCustomCert() (certPath, keyPath string, fallback bool) {
	fmt.Println()
	certPath = common.Prompt("Certificate path", "")
	keyPath = common.Prompt("Key path", "")
	if !common.FileExists(certPath) || !common.FileExists(keyPath) {
		common.Warning("Certificate files not found. Falling back to self-signed.")
		return "", "", true
	}
	return certPath, keyPath, false
}

// printTLSOptions displays TLS mode options
func printTLSOptions() {
	common.Step(3, 5, "TLS Certificate Mode")
	fmt.Println("How should HTTPS certificates be handled?")
	fmt.Println()
	fmt.Println("  1) ACME HTTP-01  - Auto cert, requires DNS pointing directly to this server")
	fmt.Println("  2) ACME DNS-01   - Auto cert via Cloudflare DNS (works behind proxy)")
	fmt.Println("  3) Custom cert   - Provide your own certificate files")
	fmt.Println("  4) HTTP only     - No HTTPS (for testing only)")
	fmt.Println("  5) Self-signed   - Works everywhere, browser shows warning (default)")
	fmt.Println()
}

// handleACMEDNSMode handles ACME DNS-01 mode configuration
func handleACMEDNSMode() (tlsMode, cfAPIToken string) {
	token, fallback := promptACMEDNS()
	if fallback {
		return TLSModeSelfSigned, ""
	}
	return TLSModeACMEDNS, token
}

// handleCustomCertMode handles custom certificate mode configuration
func handleCustomCertMode() (tlsMode, certPath, keyPath string) {
	cert, key, fallback := promptCustomCert()
	if fallback {
		return TLSModeSelfSigned, "", ""
	}
	return TLSModeCustomCert, cert, key
}

// handleTLSMode handles the selected TLS mode and returns config values
func handleTLSMode(mode string) (tlsMode, cfAPIToken, certPath, keyPath string) {
	switch mode {
	case TLSModeACMEHTTP:
		common.Info("Using ACME HTTP-01 challenge")
		return mode, "", "", ""
	case TLSModeACMEDNS:
		tlsMode, cfAPIToken = handleACMEDNSMode()
		return tlsMode, cfAPIToken, "", ""
	case TLSModeCustomCert:
		tlsMode, certPath, keyPath = handleCustomCertMode()
		return tlsMode, "", certPath, keyPath
	case TLSModeHTTPOnly:
		common.Info("Using HTTP only (no TLS)")
		return mode, "", "", ""
	default:
		common.Info("Using self-signed certificate")
		return TLSModeSelfSigned, "", "", ""
	}
}

// promptTLSMode prompts for TLS configuration
func promptTLSMode() (tlsMode, cfAPIToken, certPath, keyPath string) {
	printTLSOptions()
	mode := common.Prompt("TLS mode", "5")
	return handleTLSMode(mode)
}

// printSSHKeyPromptHeader prints the SSH key prompt header
func printSSHKeyPromptHeader() {
	common.Step(4, 5, "SSH Keys")
	fmt.Println("Add SSH public keys for server access (deploy and root users).")
	fmt.Println("Paste one key per line. Enter empty line when done.")
	fmt.Println()
	fmt.Printf("%sWARNING: If you don't add a key, you may be locked out!%s\n\n", common.Yellow, common.Reset)
}

// collectSSHKeys collects SSH keys from user input
func collectSSHKeys(maxKeys int) []string {
	var sshKeys []string
	for len(sshKeys) < maxKeys {
		key := common.Prompt("SSH key (or Enter to finish)", "")
		if key == "" {
			break
		}
		if common.IsValidSSHKey(key) {
			sshKeys = append(sshKeys, key)
			common.Success("Key added")
		} else {
			common.Error("Invalid key format. Keys should be: ssh-ed25519, ssh-rsa, or ecdsa-sha2-nistp256/384/521")
		}
	}
	return sshKeys
}

// warnNoSSHKeys warns if no SSH keys were added
func warnNoSSHKeys() {
	fmt.Println()
	common.Error("No SSH keys added! You may be locked out after reboot.")
	if !common.Confirm("Continue anyway?", false) {
		fmt.Println("Setup cancelled. Run 'juniper-host wizard' to try again.")
		os.Exit(1)
	}
}

// promptSSHKeys prompts for SSH keys
func promptSSHKeys() []string {
	const maxSSHKeys = 50
	printSSHKeyPromptHeader()
	sshKeys := collectSSHKeys(maxSSHKeys)
	if len(sshKeys) >= maxSSHKeys {
		common.Warning(fmt.Sprintf("Maximum of %d SSH keys reached.", maxSSHKeys))
	}
	if len(sshKeys) == 0 {
		warnNoSSHKeys()
	}
	return sshKeys
}

// showSummary displays configuration summary and prompts for confirmation
func showSummary(cfg wizardConfig) {
	tlsModeName := map[string]string{
		TLSModeACMEHTTP:   "ACME HTTP-01",
		TLSModeACMEDNS:    "ACME DNS-01 (Cloudflare)",
		TLSModeCustomCert: "Custom certificate",
		TLSModeHTTPOnly:   "HTTP only",
		TLSModeSelfSigned: "Self-signed",
	}[cfg.tlsMode]

	common.ClearScreen()
	fmt.Printf("%sConfiguration Summary%s\n\n", common.Bold, common.Reset)
	fmt.Printf("  Hostname: %s%s%s\n", common.Cyan, cfg.hostname, common.Reset)
	fmt.Printf("  Domain:   %s%s%s\n", common.Cyan, cfg.domain, common.Reset)
	fmt.Printf("  TLS Mode: %s%s%s\n", common.Cyan, tlsModeName, common.Reset)
	fmt.Printf("  SSH Keys: %s%d key(s)%s\n", common.Cyan, len(cfg.sshKeys), common.Reset)
	deployStr := "No"
	if cfg.deployNow {
		deployStr = "Yes"
	}
	fmt.Printf("  Deploy:   %s%s%s\n", common.Cyan, deployStr, common.Reset)
	fmt.Println()

	if !common.Confirm("Apply this configuration?", true) {
		fmt.Println("Setup cancelled. Run 'juniper-host wizard' to try again.")
		os.Exit(1)
	}
}

// backupConfig backs up the NixOS configuration
func backupConfig() {
	if err := copyFile(nixosConfig, nixosConfig+".backup"); err != nil {
		common.Error(fmt.Sprintf("Failed to backup config: %v", err))
		os.Exit(1)
	}
}

// updateNixOSConfig updates hostname and SSH keys in the config
func updateNixOSConfig(hostname string, sshKeys []string) {
	if err := updateConfig(hostname, sshKeys); err != nil {
		common.Error(fmt.Sprintf("Failed to update configuration: %v", err))
		os.Exit(1)
	}
	common.Success("Configuration updated")
}

// generateCaddyConfig generates the Caddyfile configuration
func generateCaddyConfig(cfg wizardConfig) {
	if err := generateCaddyfile(cfg.domain, cfg.tlsMode, cfg.cfAPIToken, cfg.certPath, cfg.keyPath); err != nil {
		common.Error(fmt.Sprintf("Failed to generate Caddyfile: %v", err))
		os.Exit(1)
	}
	common.Success("Caddyfile generated")
}

// rebuildNixOS rebuilds NixOS with the new configuration
func rebuildNixOS() {
	fmt.Println()
	fmt.Println("Rebuilding NixOS (this may take a minute)...")
	if err := common.Run("nixos-rebuild", "switch"); err != nil {
		common.Error("NixOS rebuild failed. Restoring backup...")
		if restoreErr := copyFile(nixosConfig+".backup", nixosConfig); restoreErr != nil {
			common.Error(fmt.Sprintf("Failed to restore backup: %v", restoreErr))
			fmt.Printf("  Manual restore: sudo cp %s.backup %s\n", nixosConfig, nixosConfig)
		} else {
			common.Success("Backup restored")
		}
		os.Exit(1)
	}
	common.Success("NixOS rebuilt successfully")
}

// applyConfiguration applies the collected configuration
func applyConfiguration(cfg wizardConfig) {
	fmt.Println()
	fmt.Printf("%sApplying configuration...%s\n\n", common.Bold, common.Reset)

	backupConfig()
	updateNixOSConfig(cfg.hostname, cfg.sshKeys)
	generateCaddyConfig(cfg)
	rebuildNixOS()

	if err := os.WriteFile(setupDoneFlag, []byte{}, 0644); err != nil {
		common.Warning(fmt.Sprintf("Failed to create setup flag: %v", err))
	}
}

// deploySite deploys the site if requested
func deploySite(deploy bool) {
	if !deploy {
		return
	}
	fmt.Println()
	fmt.Println("Deploying Juniper Bible...")
	if err := common.Run("/etc/deploy-juniper.sh"); err != nil {
		common.Warning("Site deployment failed. You can try again with: deploy-juniper")
	} else {
		common.Success("Site deployed successfully")
	}
}

// showCompletionMessage displays final success message
func showCompletionMessage(domain string) {
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
	fmt.Printf("  Admin:   %sssh root@%s%s  (for system administration)\n", common.Cyan, common.GetIP(), common.Reset)
	fmt.Println()
	fmt.Println("Useful commands:")
	fmt.Println("  deploy-juniper              - Update the site")
	fmt.Println("  sudo nixos-rebuild switch   - Apply config changes")
	fmt.Println()
}

// Run executes the setup wizard
func Run(args []string) {
	if common.FileExists(setupDoneFlag) {
		return
	}

	hostname := common.GetHostname()
	common.ClearScreen()
	common.Banner(hostname, common.GetIP(), common.GetOSVersion(), common.GetKernel())
	common.WaitForEnter("Press Enter to continue...")

	var cfg wizardConfig
	cfg.hostname = promptHostname(hostname)
	cfg.domain = promptDomain()
	cfg.tlsMode, cfg.cfAPIToken, cfg.certPath, cfg.keyPath = promptTLSMode()
	cfg.sshKeys = promptSSHKeys()

	common.Step(5, 5, "Deploy Site")
	fmt.Println("Would you like to deploy Juniper Bible now?")
	fmt.Println()
	cfg.deployNow = common.Confirm("Deploy site?", true)

	showSummary(cfg)
	applyConfiguration(cfg)
	deploySite(cfg.deployNow)
	showCompletionMessage(cfg.domain)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0600); err != nil {
		return err
	}
	// Verify backup was written successfully
	info, err := os.Stat(dst)
	if err != nil {
		return fmt.Errorf("backup verification failed: %w", err)
	}
	if info.Size() != int64(len(data)) {
		return fmt.Errorf("backup size mismatch: expected %d, got %d", len(data), info.Size())
	}
	return nil
}

// updateHostname updates the hostname in the config content
func updateHostname(content, hostname string) (string, error) {
	hostnameRe := regexp.MustCompile(`networking\.hostName = "[^"]*"`)
	escapedHostname := escapeNixString(hostname)
	newContent := hostnameRe.ReplaceAllLiteralString(content, fmt.Sprintf(`networking.hostName = "%s"`, escapedHostname))
	if newContent == content {
		return "", fmt.Errorf("failed to find hostname configuration in file")
	}
	return newContent, nil
}

// buildSSHKeysNix builds the Nix SSH keys list string
func buildSSHKeysNix(sshKeys []string) string {
	var keysList strings.Builder
	for _, key := range sshKeys {
		escapedKey := escapeNixString(key)
		keysList.WriteString(fmt.Sprintf("    \"%s\"\n", escapedKey))
	}
	return keysList.String()
}

// updateUserSSHKeys updates SSH keys for a specific user in the config
func updateUserSSHKeys(content, user, keysListStr string) string {
	var keysNix strings.Builder
	keysNix.WriteString(fmt.Sprintf("users.users.%s.openssh.authorizedKeys.keys = [\n", user))
	keysNix.WriteString(keysListStr)
	keysNix.WriteString("  ];")

	pattern := fmt.Sprintf(`users\.users\.%s\.openssh\.authorizedKeys\.keys = \[[\s\S]*?\];`, user)
	keysRe := regexp.MustCompile(pattern)
	return keysRe.ReplaceAllLiteralString(content, keysNix.String())
}

func updateConfig(hostname string, sshKeys []string) error {
	data, err := os.ReadFile(nixosConfig)
	if err != nil {
		return err
	}

	content, err := updateHostname(string(data), hostname)
	if err != nil {
		return err
	}

	if len(sshKeys) > 0 {
		beforeSSHKeys := content
		keysListStr := buildSSHKeysNix(sshKeys)
		content = updateUserSSHKeys(content, "deploy", keysListStr)
		content = updateUserSSHKeys(content, "root", keysListStr)
		if content == beforeSSHKeys {
			return fmt.Errorf("failed to find SSH key configuration sections in file")
		}
	}

	return os.WriteFile(nixosConfig, []byte(content), 0600)
}

func generateCaddyfile(domain, tlsMode, cfAPIToken, certPath, keyPath string) error {
	// Shared site configuration snippet (imported by each server block)
	siteConfigSnippet := `(site_config) {
  root * /var/www/juniperbible
  encode gzip

  # Static redirects (301) - matches _redirects
  @religion path /religion/*
  redir @religion /bible/drc/isa/42/ 301

  @licenses path /licenses/*
  redir @licenses /license/ 301

  # SPA-style rewrites for compare page clean URLs
  # Matches: /bible/compare/{bibles}/{book}/{chapter}[/{verse}][/{mode}]
  @compare_spa {
    path_regexp ^/bible/compare/[^/]+/[^/]+/[^/]+
  }
  rewrite @compare_spa /bible/compare/index.html

  file_server {
    precompressed br gzip
  }

  @static {
    path *.css *.js *.woff2 *.png *.jpg *.svg *.ico
  }
  header @static Cache-Control "public, max-age=31536000, immutable"

  @bible {
    path /bible/*
  }
  header @bible Cache-Control "public, max-age=86400"

  header {
    X-Content-Type-Options nosniff
    X-Frame-Options DENY
    Referrer-Policy strict-origin-when-cross-origin
    Permissions-Policy "camera=(), microphone=(), geolocation=()"
  }
}`

	var content string

	switch tlsMode {
	case TLSModeACMEHTTP:
		content = fmt.Sprintf(`# Juniper Bible - TLS Mode: ACME HTTP-01
{
  log {
    level ERROR
  }
}

%s

%s {
  import site_config
  header Strict-Transport-Security "max-age=31536000; includeSubDomains"
}
`, siteConfigSnippet, domain)

	case TLSModeACMEDNS:
		content = fmt.Sprintf(`# Juniper Bible - TLS Mode: ACME DNS-01 (Cloudflare)
{
  log {
    level ERROR
  }
}

%s

%s {
  tls {
    dns cloudflare %s
  }
  import site_config
  header Strict-Transport-Security "max-age=31536000; includeSubDomains"
}
`, siteConfigSnippet, domain, cfAPIToken)

	case TLSModeCustomCert:
		content = fmt.Sprintf(`# Juniper Bible - TLS Mode: Custom Certificate
{
  log {
    level ERROR
  }
}

%s

%s {
  tls %s %s
  import site_config
  header Strict-Transport-Security "max-age=31536000; includeSubDomains"
}
`, siteConfigSnippet, domain, certPath, keyPath)

	case TLSModeHTTPOnly:
		content = fmt.Sprintf(`# Juniper Bible - TLS Mode: HTTP Only
{
  log {
    level ERROR
  }
}

%s

:80 {
  import site_config
}
`, siteConfigSnippet)

	default: // TLSModeSelfSigned
		// For self-signed mode, serve HTTP without redirect (for Cloudflare proxy)
		// Direct HTTPS access uses self-signed cert
		content = fmt.Sprintf(`# Juniper Bible - TLS Mode: Self-signed
# HTTP is served without redirect (for Cloudflare proxy)
# Direct HTTPS uses self-signed certificate
{
  log {
    level ERROR
  }
}

%s

# HTTPS with self-signed certificate (direct access)
%s, :443 {
  tls internal
  import site_config
}

# HTTP - serve content (for Cloudflare proxy)
:80 {
  import site_config
}
`, siteConfigSnippet, domain)
	}

	return os.WriteFile(caddyfile, []byte(content), 0644)
}

// escapeNixString escapes special characters for Nix string literals
func escapeNixString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	s = strings.ReplaceAll(s, `$`, `\$`)
	return s
}
