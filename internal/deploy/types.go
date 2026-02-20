// Package deploy provides delta deployment functionality for Juniper Bible.
// It supports local and remote (SSH) deployments with manifest-based delta uploads.
package deploy

import (
	"time"
)

// Environment defines a deployment target.
type Environment struct {
	Name    string // Environment name (local, dev, prod)
	Target  string // SSH target (user@host) or empty for local
	Path    string // Base path on target
	KeepN   int    // Number of releases to keep
	BaseURL string // Base URL for Hugo build
}

// Options configures a deployment.
type Options struct {
	ReleaseID string // Override auto-generated release ID
	DryRun    bool   // Show what would be deployed without doing it
	Full      bool   // Force full upload (skip delta)
	NoBuild   bool   // Skip Hugo build
}

// Manifest represents a build manifest with file checksums.
type Manifest struct {
	Files     map[string]FileInfo `json:"files"`
	ReleaseID string              `json:"releaseId,omitempty"`
	BuildTime time.Time           `json:"buildTime,omitempty"`
}

// FileInfo contains file metadata.
type FileInfo struct {
	SHA256 string `json:"sha256"`
	Size   int64  `json:"size"`
}

// Delta represents the difference between local and remote manifests.
type Delta struct {
	Changed   []string // Files that are new or changed
	Unchanged []string // Files that are identical
	Deleted   []string // Files that exist remotely but not locally
}

// Release represents a deployed release.
type Release struct {
	ID        string    // Release ID
	Path      string    // Full path to release
	CreatedAt time.Time // When the release was created
	Current   bool      // Whether this is the current release
}

// Deployer defines the interface for deployment targets.
type Deployer interface {
	// FetchManifest retrieves the current manifest from the target.
	FetchManifest() (*Manifest, error)

	// CreateRelease creates a new release directory, optionally hardlinking from current.
	CreateRelease(releaseID string) error

	// UploadFull uploads all files to the release.
	UploadFull(buildDir, releaseID string) error

	// UploadDelta uploads only changed files to the release.
	UploadDelta(buildDir, releaseID string, files []string) error

	// Activate validates and activates the release via symlink swap.
	Activate(releaseID string) error

	// Cleanup removes old releases, keeping the specified number.
	Cleanup(keepN int) error

	// HealthCheck verifies the deployment was successful.
	HealthCheck(releaseID string) error

	// ListReleases returns all available releases.
	ListReleases() ([]Release, error)

	// Rollback switches to a previous release.
	Rollback(releaseID string) error
}
