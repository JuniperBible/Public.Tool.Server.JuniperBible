package common

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

const (
	RepoBase   = "https://raw.githubusercontent.com/JuniperBible/Website.Server.JuniperBible.org/main"
	ReleaseURL = "https://github.com/JuniperBible/Website.Server.JuniperBible.org/releases/latest/download/site.tar.xz"
)

// Pre-compiled regex patterns for validation
var (
	// Note: ssh-dss (DSA) is excluded as it's deprecated and limited to 1024 bits
	sshKeyPattern = regexp.MustCompile(`^(ssh-rsa|ssh-ed25519|ecdsa-sha2-nistp256|ecdsa-sha2-nistp384|ecdsa-sha2-nistp521)\s+[A-Za-z0-9+/]+=*(\s+[^\s].*)?$`)
	diskPathPattern = regexp.MustCompile(`^/dev/(nvme\d+n\d+|[svx]d[a-z]+|loop\d+)$`)
	hostnamePattern = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
	domainPattern   = regexp.MustCompile(`^([a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?\.)*[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`)
)

// GetHostname returns the system hostname
func GetHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

// GetIP returns the first IP address
func GetIP() string {
	out, err := RunOutput("hostname", "-I")
	if err != nil {
		return "N/A"
	}
	parts := strings.Fields(out)
	if len(parts) > 0 {
		return parts[0]
	}
	return "N/A"
}

// GetOSVersion returns the NixOS version
func GetOSVersion() string {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return "unknown"
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "VERSION_ID=") {
			return strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
		}
	}
	if err := scanner.Err(); err != nil {
		return "unknown"
	}
	return "unknown"
}

// GetKernel returns the kernel version
func GetKernel() string {
	out, err := RunOutput("uname", "-r")
	if err != nil {
		return "unknown"
	}
	return out
}

// DetectDisk auto-detects the primary disk
func DetectDisk() string {
	disks := []string{"/dev/vda", "/dev/sda", "/dev/nvme0n1", "/dev/xvda"}
	for _, disk := range disks {
		if BlockDeviceExists(disk) {
			return disk
		}
	}
	return ""
}

// GetPartitions returns the partition paths for a disk
// Returns: bios_grub (1), ESP (2), root (3)
func GetPartitions(disk string) (biosGrub, esp, root string) {
	if strings.Contains(disk, "nvme") {
		return disk + "p1", disk + "p2", disk + "p3"
	}
	return disk + "1", disk + "2", disk + "3"
}

// MaxDownloadSize is the maximum file size for downloads (100MB)
const MaxDownloadSize = 100 * 1024 * 1024

// DownloadFile downloads a file from URL to destination
func DownloadFile(url, dest string) error {
	// Check if destination is a symlink (prevent symlink attacks)
	if info, err := os.Lstat(dest); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("destination is a symlink: %s", dest)
		}
	}

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{StatusCode: resp.StatusCode, URL: url}
	}

	// Create file with restrictive permissions
	out, err := os.OpenFile(dest, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}

	// Limit download size to prevent DoS
	limitedReader := io.LimitReader(resp.Body, MaxDownloadSize)
	_, copyErr := io.Copy(out, limitedReader)
	closeErr := out.Close()

	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// HTTPError represents an HTTP error
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("HTTP %d from %s", e.StatusCode, e.URL)
}

// MaxSSHKeyLength is the maximum allowed SSH key length
const MaxSSHKeyLength = 8192

// IsValidSSHKey validates an SSH public key format
func IsValidSSHKey(key string) bool {
	key = strings.TrimSpace(key)
	// Reject keys with newlines (multi-key injection)
	if strings.ContainsAny(key, "\n\r") {
		return false
	}
	// Reject extremely long keys
	if len(key) > MaxSSHKeyLength {
		return false
	}
	// Validate format: type + space + base64 + optional comment
	return sshKeyPattern.MatchString(key)
}

// IsValidDiskPath validates a disk device path
func IsValidDiskPath(path string) bool {
	// Match standard Linux disk paths: /dev/vda, /dev/sda, /dev/sdaa, /dev/nvme0n1, /dev/xvda, etc.
	return diskPathPattern.MatchString(path)
}

// IsValidHostname validates a hostname format
func IsValidHostname(hostname string) bool {
	// RFC 1123: max 63 chars per label
	if len(hostname) == 0 || len(hostname) > 63 {
		return false
	}
	// RFC 1123: alphanumeric and hyphens, cannot start/end with hyphen
	return hostnamePattern.MatchString(hostname)
}

// IsValidDomain validates a domain name format
func IsValidDomain(domain string) bool {
	if domain == "localhost" {
		return true
	}
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}
	// RFC 1035: labels separated by dots, each label alphanumeric with hyphens
	return domainPattern.MatchString(domain)
}
