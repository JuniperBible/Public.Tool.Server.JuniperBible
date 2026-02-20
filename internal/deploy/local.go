package deploy

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// LocalDeployer implements Deployer for local filesystem deployments.
type LocalDeployer struct {
	basePath string
}

// NewLocalDeployer creates a new local deployer.
func NewLocalDeployer(basePath string) *LocalDeployer {
	return &LocalDeployer{basePath: basePath}
}

// releasesDir returns the path to the releases directory.
func (d *LocalDeployer) releasesDir() string {
	return filepath.Join(d.basePath, "releases")
}

// releaseDir returns the path to a specific release.
func (d *LocalDeployer) releaseDir(releaseID string) string {
	return filepath.Join(d.releasesDir(), releaseID)
}

// currentLink returns the path to the current symlink.
func (d *LocalDeployer) currentLink() string {
	return filepath.Join(d.basePath, "current")
}

// FetchManifest retrieves the current manifest from the local deployment.
func (d *LocalDeployer) FetchManifest() (*Manifest, error) {
	manifestPath := filepath.Join(d.currentLink(), "build-manifest.json")
	return ReadManifest(manifestPath)
}

// CreateRelease creates a new release directory with hardlinks from current.
func (d *LocalDeployer) CreateRelease(releaseID string) error {
	releaseDir := d.releaseDir(releaseID)
	currentLink := d.currentLink()

	// Check if current exists
	target, err := os.Readlink(currentLink)
	if err == nil {
		// Resolve relative symlink
		if !filepath.IsAbs(target) {
			target = filepath.Join(filepath.Dir(currentLink), target)
		}

		// Use cp -al for hardlink copy
		cmd := exec.Command("cp", "-al", target, releaseDir)
		if output, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("hardlink copy failed: %s: %w", output, err)
		}
		return nil
	}

	// First release - create empty directory
	if err := os.MkdirAll(releaseDir, 0755); err != nil {
		return fmt.Errorf("create release dir: %w", err)
	}
	return nil
}

// UploadFull copies all files to the release directory.
func (d *LocalDeployer) UploadFull(buildDir, releaseID string) error {
	releaseDir := d.releaseDir(releaseID)

	// Walk and copy all files
	return filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(buildDir, path)
		if err != nil {
			return err
		}

		dstPath := filepath.Join(releaseDir, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		return copyFile(path, dstPath)
	})
}

// UploadDelta copies only changed files to the release directory.
func (d *LocalDeployer) UploadDelta(buildDir, releaseID string, files []string) error {
	releaseDir := d.releaseDir(releaseID)

	for _, file := range files {
		src := filepath.Join(buildDir, file)
		dst := filepath.Join(releaseDir, file)

		// Ensure parent directory exists
		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		if err := copyFile(src, dst); err != nil {
			return fmt.Errorf("copy %s: %w", file, err)
		}
	}

	return nil
}

// Activate validates and activates the release via symlink swap.
func (d *LocalDeployer) Activate(releaseID string) error {
	releaseDir := d.releaseDir(releaseID)
	currentLink := d.currentLink()
	tmpLink := currentLink + ".new"

	// Validate required files
	requiredFiles := []string{"healthz.json", "index.html", "sw.js"}
	for _, f := range requiredFiles {
		path := filepath.Join(releaseDir, f)
		if _, err := os.Stat(path); err != nil {
			// Cleanup on failure
			os.RemoveAll(releaseDir)
			return fmt.Errorf("validation failed: %s missing", f)
		}
	}

	// Remove temp link if exists
	os.Remove(tmpLink)

	// Create new symlink (use relative path for portability)
	relPath, err := filepath.Rel(filepath.Dir(currentLink), releaseDir)
	if err != nil {
		relPath = releaseDir // Fall back to absolute
	}

	if err := os.Symlink(relPath, tmpLink); err != nil {
		return fmt.Errorf("create symlink: %w", err)
	}

	// Atomic rename
	if err := os.Rename(tmpLink, currentLink); err != nil {
		os.Remove(tmpLink)
		return fmt.Errorf("activate release: %w", err)
	}

	return nil
}

// Cleanup removes old releases, keeping the specified number.
func (d *LocalDeployer) Cleanup(keepN int) error {
	releases, err := d.ListReleases()
	if err != nil {
		return err
	}

	if len(releases) <= keepN {
		return nil
	}

	// Sort by creation time (newest first) - ListReleases returns sorted
	for _, release := range releases[keepN:] {
		if release.Current {
			continue // Never delete current
		}
		if err := os.RemoveAll(release.Path); err != nil {
			return fmt.Errorf("remove %s: %w", release.ID, err)
		}
	}

	return nil
}

// HealthCheck verifies the deployment was successful.
func (d *LocalDeployer) HealthCheck(releaseID string) error {
	healthzPath := filepath.Join(d.currentLink(), "healthz.json")
	manifest, err := os.ReadFile(healthzPath)
	if err != nil {
		return fmt.Errorf("read healthz.json: %w", err)
	}

	if !strings.Contains(string(manifest), releaseID) {
		return fmt.Errorf("release ID %s not found in healthz.json", releaseID)
	}

	return nil
}

// ListReleases returns all available releases, sorted by creation time (newest first).
func (d *LocalDeployer) ListReleases() ([]Release, error) {
	entries, err := os.ReadDir(d.releasesDir())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	// Get current release
	currentTarget, _ := os.Readlink(d.currentLink())
	if !filepath.IsAbs(currentTarget) {
		currentTarget = filepath.Join(filepath.Dir(d.currentLink()), currentTarget)
	}

	var releases []Release
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		releasePath := filepath.Join(d.releasesDir(), entry.Name())
		releases = append(releases, Release{
			ID:        entry.Name(),
			Path:      releasePath,
			CreatedAt: info.ModTime(),
			Current:   releasePath == currentTarget,
		})
	}

	// Sort by creation time (newest first)
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].CreatedAt.After(releases[j].CreatedAt)
	})

	return releases, nil
}

// Rollback switches to a previous release.
func (d *LocalDeployer) Rollback(releaseID string) error {
	// If no release ID specified, use the previous one
	if releaseID == "" {
		releases, err := d.ListReleases()
		if err != nil {
			return err
		}

		// Find the previous (non-current) release
		for _, r := range releases {
			if !r.Current {
				releaseID = r.ID
				break
			}
		}

		if releaseID == "" {
			return fmt.Errorf("no previous release found")
		}
	}

	// Verify release exists
	releaseDir := d.releaseDir(releaseID)
	if _, err := os.Stat(releaseDir); err != nil {
		return fmt.Errorf("release %s not found", releaseID)
	}

	return d.Activate(releaseID)
}

// copyFile copies a file from src to dst.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, srcInfo.Mode())
	if err != nil {
		return err
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return err
	}

	// Preserve modification time
	return os.Chtimes(dst, time.Now(), srcInfo.ModTime())
}
