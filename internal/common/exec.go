package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Run executes a command and streams output to stdout/stderr
func Run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	return cmd.Run()
}

// RunQuiet executes a command without output
func RunQuiet(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Run()
}

// RunOutput executes a command and returns its output
func RunOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

// RunWithOutput executes a command and captures output while also displaying it
func RunWithOutput(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", err
	}

	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return "", err
	}

	var output strings.Builder
	reader := bufio.NewReader(stdout)
	for {
		line, err := reader.ReadString('\n')
		if line != "" {
			fmt.Print(line)
			output.WriteString(line)
		}
		if err != nil {
			if err != io.EOF {
				return output.String(), err
			}
			break
		}
	}

	return output.String(), cmd.Wait()
}

// IsRoot checks if running as root
func IsRoot() bool {
	return os.Geteuid() == 0
}

// FileExists checks if a file exists
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// BlockDeviceExists checks if a block device exists
func BlockDeviceExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeDevice != 0
}

// IsMounted checks if a path is a mountpoint
func IsMounted(path string) bool {
	err := RunQuiet("mountpoint", "-q", path)
	return err == nil
}

// RunWithProgress runs a command with a progress indicator (dots every 5 seconds)
// Use for long-running commands like nixos-install that may take 10-30 minutes
func RunWithProgress(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	// Start the command
	if err := cmd.Start(); err != nil {
		return err
	}

	// Progress indicator goroutine with proper synchronization
	done := make(chan struct{})
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		defer func() {
			if r := recover(); r != nil {
				fmt.Fprintf(os.Stderr, "\nWarning: progress indicator error: %v\n", r)
			}
		}()
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				fmt.Print(".")
			}
		}
	}()

	// Wait for command to complete
	err := cmd.Wait()
	close(done)
	<-finished // Wait for goroutine to exit
	fmt.Println() // Newline after progress dots

	return err
}
