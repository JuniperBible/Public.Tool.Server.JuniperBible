package common

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
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
		if err != nil {
			if err != io.EOF {
				return output.String(), err
			}
			break
		}
		fmt.Print(line)
		output.WriteString(line)
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
