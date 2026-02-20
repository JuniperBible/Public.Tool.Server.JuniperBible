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

// newDeployer creates the appropriate deployer for the environment.
func newDeployer(env Environment) Deployer {
	if env.Target == "" {
		return NewLocalDeployer(env.Path)
	}
	return NewRemoteDeployer(env.Target, env.Path)
}

// buildAndGenerateManifest builds Hugo and generates manifest.
func buildAndGenerateManifest(releaseID string, env Environment, noBuild bool) (*Manifest, error) {
	if !noBuild {
		fmt.Println("==> Building Hugo...")
		if err := BuildHugo(releaseID, env.BaseURL); err != nil {
			return nil, fmt.Errorf("hugo build failed: %w", err)
		}
		fmt.Println()
	}

	fmt.Println("==> Generating build manifest...")
	manifest, err := GenerateManifestWithWorkers("public", releaseID, DefaultWorkers)
	if err != nil {
		return nil, fmt.Errorf("manifest generation failed: %w", err)
	}

	manifestPath := filepath.Join("public", "build-manifest.json")
	if err := WriteManifest(manifest, manifestPath); err != nil {
		return nil, fmt.Errorf("write manifest: %w", err)
	}
	fmt.Printf("    %d files hashed\n", len(manifest.Files))
	fmt.Println()

	return manifest, nil
}

// fetchRemoteManifest fetches the manifest from the remote server.
func fetchRemoteManifest(deployer Deployer) *Manifest {
	fmt.Println("==> Fetching remote manifest...")
	manifest, err := deployer.FetchManifest()
	if err != nil {
		fmt.Printf("    No previous manifest (first deploy)\n")
		manifest = &Manifest{Files: make(map[string]FileInfo)}
	} else {
		fmt.Printf("    Previous release: %s\n", manifest.ReleaseID)
	}
	fmt.Println()
	return manifest
}

// printDeltaStats prints delta statistics.
func printDeltaStats(delta *Delta, localManifest *Manifest) {
	fmt.Println("==> Calculating delta...")
	fmt.Printf("    Changed:   %d files\n", len(delta.Changed))
	fmt.Printf("    Unchanged: %d files\n", len(delta.Unchanged))
	if len(delta.Deleted) > 0 {
		fmt.Printf("    Deleted:   %d files (will remain in hardlinked release)\n", len(delta.Deleted))
	}

	changedSize := DeltaSize(localManifest, delta.Changed)
	totalSize := localManifest.TotalSize()
	fmt.Printf("    Delta:     %.2f MB (%.1f%% of %.2f MB total)\n",
		float64(changedSize)/(1024*1024),
		float64(changedSize)/float64(totalSize)*100,
		float64(totalSize)/(1024*1024))
	fmt.Println()
}

// uploadFiles uploads files to the release directory.
func uploadFiles(deployer Deployer, releaseID string, delta *Delta, remoteManifest *Manifest, full bool) error {
	if full || len(remoteManifest.Files) == 0 {
		fmt.Println("==> Uploading all files...")
		return deployer.UploadFull("public", releaseID)
	}
	if len(delta.Changed) > 0 {
		fmt.Println("==> Uploading changed files...")
		return deployer.UploadDelta("public", releaseID, delta.Changed)
	}
	fmt.Println("==> No files changed, skipping upload")
	return nil
}

// printDeployHeader prints deployment info header
func printDeployHeader(env Environment, releaseID string) {
	fmt.Printf("==> Deploying to %s\n", env.Name)
	fmt.Printf("    Release: %s\n", releaseID)
	fmt.Printf("    Target:  %s\n", targetDescription(env))
	fmt.Println()
}

// createReleaseDir creates the release directory
func createReleaseDir(deployer Deployer, releaseID string) error {
	fmt.Println("==> Creating release directory...")
	if err := deployer.CreateRelease(releaseID); err != nil {
		return fmt.Errorf("create release: %w", err)
	}
	return nil
}

// activateRelease activates the release and prints status
func activateRelease(deployer Deployer, releaseID string) error {
	fmt.Println("==> Activating release...")
	if err := deployer.Activate(releaseID); err != nil {
		return fmt.Errorf("activate: %w", err)
	}
	fmt.Println()
	return nil
}

// cleanupOldReleases cleans up old releases
func cleanupOldReleases(deployer Deployer, keepN int) {
	fmt.Printf("==> Cleaning old releases (keeping %d)...\n", keepN)
	if err := deployer.Cleanup(keepN); err != nil {
		fmt.Printf("    Warning: cleanup failed: %v\n", err)
	}
	fmt.Println()
}

// runHealthCheck runs the health check
func runHealthCheck(deployer Deployer, releaseID string) {
	fmt.Println("==> Health check...")
	if err := deployer.HealthCheck(releaseID); err != nil {
		fmt.Printf("    Warning: %v\n", err)
	} else {
		fmt.Println("    OK")
	}
	fmt.Println()
}

// executeDeployment performs the actual deployment steps
func executeDeployment(deployer Deployer, releaseID string, delta *Delta, remoteManifest *Manifest, env Environment, full bool) error {
	if err := createReleaseDir(deployer, releaseID); err != nil {
		return err
	}
	if err := uploadFiles(deployer, releaseID, delta, remoteManifest, full); err != nil {
		return fmt.Errorf("upload: %w", err)
	}
	fmt.Println()
	if err := activateRelease(deployer, releaseID); err != nil {
		return err
	}
	cleanupOldReleases(deployer, env.KeepN)
	runHealthCheck(deployer, releaseID)
	return nil
}

// printDryRunChanges prints the files that would be changed in dry run mode
func printDryRunChanges(delta *Delta) {
	fmt.Println("==> Dry run - no changes made")
	for _, f := range delta.Changed {
		fmt.Printf("  + %s\n", f)
	}
}

// Deploy performs a deployment to the given environment.
func Deploy(env Environment, opts Options) error {
	releaseID := opts.ReleaseID
	if releaseID == "" {
		releaseID = GenerateReleaseID()
	}

	printDeployHeader(env, releaseID)

	localManifest, err := buildAndGenerateManifest(releaseID, env, opts.NoBuild)
	if err != nil {
		return err
	}

	deployer := newDeployer(env)
	remoteManifest := fetchRemoteManifest(deployer)
	delta := CalculateDelta(localManifest, remoteManifest)
	printDeltaStats(delta, localManifest)

	if opts.DryRun {
		printDryRunChanges(delta)
		return nil
	}

	if err := executeDeployment(deployer, releaseID, delta, remoteManifest, env, opts.Full); err != nil {
		return err
	}
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

// printReleaseList prints the list of releases
func printReleaseList(releases []Release, env Environment) {
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
}

// ListReleases lists releases on the target.
func ListReleases(env Environment) error {
	deployer := newDeployer(env)
	releases, err := deployer.ListReleases()
	if err != nil {
		return err
	}
	if len(releases) == 0 {
		fmt.Println("No releases found")
		return nil
	}
	printReleaseList(releases, env)
	return nil
}

// findPreviousRelease finds the first non-current release
func findPreviousRelease(deployer Deployer) (string, error) {
	releases, err := deployer.ListReleases()
	if err != nil {
		return "", err
	}
	for _, r := range releases {
		if !r.Current {
			return r.ID, nil
		}
	}
	return "", fmt.Errorf("no previous release found")
}

// Rollback switches to a previous release.
func Rollback(env Environment, releaseID string) error {
	deployer := newDeployer(env)

	targetID := releaseID
	if targetID == "" {
		var err error
		targetID, err = findPreviousRelease(deployer)
		if err != nil {
			return err
		}
	}

	fmt.Printf("==> Rolling back to %s on %s...\n", targetID, env.Name)

	if err := deployer.Rollback(targetID); err != nil {
		return err
	}

	fmt.Printf("Done! Rolled back to %s\n", targetID)
	return nil
}

// printLocalStatus prints status for local deployment
func printLocalStatus(deployer *LocalDeployer) error {
	releases, err := deployer.ListReleases()
	if err != nil {
		return err
	}

	for _, r := range releases {
		if r.Current {
			fmt.Printf("Current release: %s\n", r.ID)
			fmt.Printf("Deployed at:     %s\n", r.CreatedAt.Format("2006-01-02 15:04:05"))
			healthzPath := filepath.Join(r.Path, "healthz.json")
			if data, err := os.ReadFile(healthzPath); err == nil {
				fmt.Printf("\nhealthz.json:\n%s\n", data)
			}
			return nil
		}
	}
	fmt.Println("No current release")
	return nil
}

// printRemoteStatus prints status for remote deployment
func printRemoteStatus(deployer *RemoteDeployer) {
	currentID, err := deployer.GetCurrentRelease()
	if err != nil || currentID == "" {
		fmt.Println("No current release")
		return
	}

	fmt.Printf("Current release: %s\n", currentID)
	if healthz, err := deployer.GetHealthz(); err == nil {
		fmt.Printf("\nhealthz.json:\n%s\n", healthz)
	}
}

// Status shows the current deployment status.
func Status(env Environment) error {
	fmt.Printf("Environment: %s\n", env.Name)
	fmt.Printf("Target:      %s\n", targetDescription(env))
	fmt.Println()

	if env.Target == "" {
		return printLocalStatus(NewLocalDeployer(env.Path))
	}
	printRemoteStatus(NewRemoteDeployer(env.Target, env.Path))
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
