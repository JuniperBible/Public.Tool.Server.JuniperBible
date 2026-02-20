package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// BuildHugo runs Hugo with the given release ID and base URL.
func BuildHugo(releaseID, baseURL string) error {
	args := []string{"--minify"}

	if baseURL != "" {
		args = append(args, "--baseURL", baseURL)
	}

	// Use Hugo cache for faster builds
	cacheDir := os.ExpandEnv("$HOME/.cache/hugo")
	args = append(args, "--cacheDir", cacheDir)

	cmd := exec.Command("hugo", args...)
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("RELEASE_ID=%s", releaseID),
		fmt.Sprintf("GOMAXPROCS=%d", runtime.NumCPU()),
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// BuildHugoWithSitemaps runs Hugo and generates sitemaps.
func BuildHugoWithSitemaps(releaseID, baseURL string) error {
	if err := BuildHugo(releaseID, baseURL); err != nil {
		return err
	}

	// Generate sitemaps
	cmd := exec.Command("./scripts/generate-sitemaps.sh", "public", baseURL)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// RestoreBibles restores the full bibles.json before building.
func RestoreBibles() error {
	cmd := exec.Command("./scripts/filter-bibles.sh", "restore")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
