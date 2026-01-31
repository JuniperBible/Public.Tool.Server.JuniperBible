package common

import (
	"bufio"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	RepoBase   = "https://raw.githubusercontent.com/JuniperBible/Website.Server.JuniperBible.org/main"
	ReleaseURL = "https://github.com/JuniperBible/Website.Server.JuniperBible.org/releases/latest/download/site.tar.xz"
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
func GetPartitions(disk string) (part1, part2 string) {
	if strings.Contains(disk, "nvme") {
		return disk + "p1", disk + "p2"
	}
	return disk + "1", disk + "2"
}

// DownloadFile downloads a file from URL to destination
func DownloadFile(url, dest string) error {
	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &HTTPError{StatusCode: resp.StatusCode, URL: url}
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

// HTTPError represents an HTTP error
type HTTPError struct {
	StatusCode int
	URL        string
}

func (e *HTTPError) Error() string {
	return "HTTP " + string(rune(e.StatusCode)) + " from " + e.URL
}

// IsValidSSHKey validates an SSH public key format
func IsValidSSHKey(key string) bool {
	key = strings.TrimSpace(key)
	return strings.HasPrefix(key, "ssh-") || strings.HasPrefix(key, "ecdsa-")
}
