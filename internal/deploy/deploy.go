package deploy

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const (
	// DefaultWorkers is the number of parallel workers for file hashing.
	DefaultWorkers = 11
)

// Deploy performs a deployment to the given environment.
func Deploy(env Environment, opts Options) error {
	// Generate release ID if not provided
	releaseID := opts.ReleaseID
	if releaseID == "" {
		releaseID = GenerateReleaseID()
	}

	fmt.Printf("==> Deploying to %s\n", env.Name)
	fmt.Printf("    Release: %s\n", releaseID)
	fmt.Printf("    Target:  %s\n", targetDescription(env))
	fmt.Println()

	// Build Hugo (unless --no-build)
	if !opts.NoBuild {
		fmt.Println("==> Building Hugo...")
		if err := BuildHugo(releaseID, env.BaseURL); err != nil {
			return fmt.Errorf("hugo build failed: %w", err)
		}
		fmt.Println()
	}

	// Generate manifest
	fmt.Println("==> Generating build manifest...")
	localManifest, err := GenerateManifestWithWorkers("public", releaseID, DefaultWorkers)
	if err != nil {
		return fmt.Errorf("manifest generation failed: %w", err)
	}

	// Write manifest to build dir
	manifestPath := filepath.Join("public", "build-manifest.json")
	if err := WriteManifest(localManifest, manifestPath); err != nil {
		return fmt.Errorf("write manifest: %w", err)
	}
	fmt.Printf("    %d files hashed\n", len(localManifest.Files))
	fmt.Println()

	// Get deployer
	var deployer Deployer
	if env.Target == "" {
		deployer = NewLocalDeployer(env.Path)
	} else {
		deployer = NewRemoteDeployer(env.Target, env.Path)
	}

	// Fetch remote manifest
	fmt.Println("==> Fetching remote manifest...")
	remoteManifest, err := deployer.FetchManifest()
	if err != nil {
		fmt.Printf("    No previous manifest (first deploy)\n")
		remoteManifest = &Manifest{Files: make(map[string]FileInfo)}
	} else {
		fmt.Printf("    Previous release: %s\n", remoteManifest.ReleaseID)
	}
	fmt.Println()

	// Calculate delta
	fmt.Println("==> Calculating delta...")
	delta := CalculateDelta(localManifest, remoteManifest)
	fmt.Printf("    Changed:   %d files\n", len(delta.Changed))
	fmt.Printf("    Unchanged: %d files\n", len(delta.Unchanged))
	if len(delta.Deleted) > 0 {
		fmt.Printf("    Deleted:   %d files (will remain in hardlinked release)\n", len(delta.Deleted))
	}

	// Calculate sizes
	changedSize := DeltaSize(localManifest, delta.Changed)
	totalSize := localManifest.TotalSize()
	fmt.Printf("    Delta:     %.2f MB (%.1f%% of %.2f MB total)\n",
		float64(changedSize)/(1024*1024),
		float64(changedSize)/float64(totalSize)*100,
		float64(totalSize)/(1024*1024))
	fmt.Println()

	// Dry run - stop here
	if opts.DryRun {
		fmt.Println("==> Dry run - no changes made")
		if len(delta.Changed) > 0 {
			fmt.Println("\nChanged files:")
			for _, f := range delta.Changed {
				fmt.Printf("  + %s\n", f)
			}
		}
		return nil
	}

	// Create release with hardlinks
	fmt.Println("==> Creating release directory...")
	if err := deployer.CreateRelease(releaseID); err != nil {
		return fmt.Errorf("create release: %w", err)
	}

	// Upload delta (or full if --full or first deploy)
	if opts.Full || len(remoteManifest.Files) == 0 {
		fmt.Println("==> Uploading all files...")
		if err := deployer.UploadFull("public", releaseID); err != nil {
			return fmt.Errorf("upload: %w", err)
		}
	} else if len(delta.Changed) > 0 {
		fmt.Println("==> Uploading changed files...")
		if err := deployer.UploadDelta("public", releaseID, delta.Changed); err != nil {
			return fmt.Errorf("upload delta: %w", err)
		}
	} else {
		fmt.Println("==> No files changed, skipping upload")
	}
	fmt.Println()

	// Validate and activate
	fmt.Println("==> Activating release...")
	if err := deployer.Activate(releaseID); err != nil {
		return fmt.Errorf("activate: %w", err)
	}
	fmt.Println()

	// Cleanup old releases
	fmt.Printf("==> Cleaning old releases (keeping %d)...\n", env.KeepN)
	if err := deployer.Cleanup(env.KeepN); err != nil {
		fmt.Printf("    Warning: cleanup failed: %v\n", err)
	}
	fmt.Println()

	// Health check
	fmt.Println("==> Health check...")
	if err := deployer.HealthCheck(releaseID); err != nil {
		fmt.Printf("    Warning: %v\n", err)
	} else {
		fmt.Println("    OK")
	}
	fmt.Println()

	fmt.Printf("Done! Release %s is now live.\n", releaseID)
	return nil
}

// GenerateReleaseID creates a release ID in format YYYYMMDD-HHMMSS-{git_hash}.
func GenerateReleaseID() string {
	timestamp := time.Now().UTC().Format("20060102-150405")

	// Get git hash
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	output, err := cmd.Output()
	if err != nil {
		return timestamp
	}

	gitHash := strings.TrimSpace(string(output))
	return fmt.Sprintf("%s-%s", timestamp, gitHash)
}

// targetDescription returns a human-readable description of the target.
func targetDescription(env Environment) string {
	if env.Target == "" {
		return fmt.Sprintf("local (%s)", env.Path)
	}
	return fmt.Sprintf("%s:%s", env.Target, env.Path)
}

// ListReleases lists releases on the target.
func ListReleases(env Environment) error {
	var deployer Deployer
	if env.Target == "" {
		deployer = NewLocalDeployer(env.Path)
	} else {
		deployer = NewRemoteDeployer(env.Target, env.Path)
	}

	releases, err := deployer.ListReleases()
	if err != nil {
		return err
	}

	if len(releases) == 0 {
		fmt.Println("No releases found")
		return nil
	}

	fmt.Printf("Releases on %s:\n\n", targetDescription(env))
	for _, r := range releases {
		current := ""
		if r.Current {
			current = " (current)"
		}
		fmt.Printf("  %s  %s%s\n",
			r.CreatedAt.Format("2006-01-02 15:04:05"),
			r.ID,
			current,
		)
	}

	return nil
}

// Rollback switches to a previous release.
func Rollback(env Environment, releaseID string) error {
	var deployer Deployer
	if env.Target == "" {
		deployer = NewLocalDeployer(env.Path)
	} else {
		deployer = NewRemoteDeployer(env.Target, env.Path)
	}

	targetID := releaseID
	if releaseID == "" {
		// Find previous release
		releases, err := deployer.ListReleases()
		if err != nil {
			return err
		}

		for _, r := range releases {
			if !r.Current {
				targetID = r.ID
				break
			}
		}

		if targetID == "" {
			return fmt.Errorf("no previous release found")
		}
	}

	fmt.Printf("==> Rolling back to %s on %s...\n", targetID, env.Name)

	if err := deployer.Rollback(targetID); err != nil {
		return err
	}

	fmt.Printf("Done! Rolled back to %s\n", targetID)
	return nil
}

// Status shows the current deployment status.
func Status(env Environment) error {
	fmt.Printf("Environment: %s\n", env.Name)
	fmt.Printf("Target:      %s\n", targetDescription(env))
	fmt.Println()

	if env.Target == "" {
		// Local status
		deployer := NewLocalDeployer(env.Path)
		releases, err := deployer.ListReleases()
		if err != nil {
			return err
		}

		for _, r := range releases {
			if r.Current {
				fmt.Printf("Current release: %s\n", r.ID)
				fmt.Printf("Deployed at:     %s\n", r.CreatedAt.Format("2006-01-02 15:04:05"))

				// Read healthz.json
				healthzPath := filepath.Join(r.Path, "healthz.json")
				if data, err := os.ReadFile(healthzPath); err == nil {
					fmt.Printf("\nhealthz.json:\n%s\n", data)
				}
				return nil
			}
		}

		fmt.Println("No current release")
	} else {
		// Remote status
		deployer := NewRemoteDeployer(env.Target, env.Path)

		currentID, err := deployer.GetCurrentRelease()
		if err != nil || currentID == "" {
			fmt.Println("No current release")
			return nil
		}

		fmt.Printf("Current release: %s\n", currentID)

		healthz, err := deployer.GetHealthz()
		if err == nil {
			fmt.Printf("\nhealthz.json:\n%s\n", healthz)
		}
	}

	return nil
}

// GenerateManifestOnly generates a build manifest without deploying.
func GenerateManifestOnly(buildDir, releaseID string) error {
	if releaseID == "" {
		releaseID = GenerateReleaseID()
	}

	fmt.Println("==> Generating build manifest...")
	manifest, err := GenerateManifestWithWorkers(buildDir, releaseID, DefaultWorkers)
	if err != nil {
		return err
	}

	manifestPath := filepath.Join(buildDir, "build-manifest.json")
	if err := WriteManifest(manifest, manifestPath); err != nil {
		return err
	}

	fmt.Printf("Manifest written to %s\n", manifestPath)
	fmt.Printf("  Files: %d\n", len(manifest.Files))
	fmt.Printf("  Size:  %.2f MB\n", float64(manifest.TotalSize())/(1024*1024))

	return nil
}
