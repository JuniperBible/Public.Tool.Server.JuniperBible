package deploy

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ulikunitz/xz"
)

// RemoteDeployer implements Deployer for SSH-based deployments.
type RemoteDeployer struct {
	host     string // user@host
	basePath string // /var/www/juniperbible
}

// NewRemoteDeployer creates a new remote deployer.
func NewRemoteDeployer(host, basePath string) *RemoteDeployer {
	return &RemoteDeployer{
		host:     host,
		basePath: basePath,
	}
}

// releasesDir returns the path to the releases directory.
func (d *RemoteDeployer) releasesDir() string {
	return filepath.Join(d.basePath, "releases")
}

// releaseDir returns the path to a specific release.
func (d *RemoteDeployer) releaseDir(releaseID string) string {
	return filepath.Join(d.releasesDir(), releaseID)
}

// currentLink returns the path to the current symlink.
func (d *RemoteDeployer) currentLink() string {
	return filepath.Join(d.basePath, "current")
}

// ssh runs a command on the remote host.
func (d *RemoteDeployer) ssh(script string) ([]byte, error) {
	cmd := exec.Command("ssh", d.host, script)
	return cmd.CombinedOutput()
}

// sshStream runs a command on the remote host with stdin streaming.
func (d *RemoteDeployer) sshStream(script string) (*exec.Cmd, io.WriteCloser, error) {
	cmd := exec.Command("ssh", d.host, script)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}

	if err := cmd.Start(); err != nil {
		stdin.Close()
		return nil, nil, err
	}

	return cmd, stdin, nil
}

// FetchManifest retrieves the current manifest from the remote server.
func (d *RemoteDeployer) FetchManifest() (*Manifest, error) {
	manifestPath := filepath.Join(d.currentLink(), "build-manifest.json")
	output, err := d.ssh(fmt.Sprintf("cat '%s' 2>/dev/null", manifestPath))
	if err != nil {
		return nil, fmt.Errorf("fetch manifest: %w", err)
	}

	var m Manifest
	if err := json.Unmarshal(output, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}

	return &m, nil
}

// CreateRelease creates a new release directory.
// Uses hardlink copy from current release for delta deploy efficiency.
// Hardlinks let unchanged files share disk space while changed files
// are replaced via unlink+extract (see UploadDelta).
func (d *RemoteDeployer) CreateRelease(releaseID string) error {
	releaseDir := d.releaseDir(releaseID)
	currentLink := d.currentLink()

	// Use cp -al to create hardlink copy if current exists, else mkdir
	// Note: We use $(readlink -f ...) to resolve symlinks, otherwise cp -al
	// would copy the symlink itself instead of the directory contents
	script := fmt.Sprintf(`
		set -e
		if [ -d '%s' ]; then
			cp -al "$(readlink -f '%s')" '%s'
		else
			mkdir -p '%s'
		fi
	`, currentLink, currentLink, releaseDir, releaseDir)

	output, err := d.ssh(script)
	if err != nil {
		return fmt.Errorf("create release: %s: %w", output, err)
	}

	return nil
}

// UploadFull uploads all files to the release directory.
func (d *RemoteDeployer) UploadFull(buildDir, releaseID string) error {
	// Collect all files
	var files []string
	err := filepath.Walk(buildDir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		relPath, _ := filepath.Rel(buildDir, path)
		files = append(files, relPath)
		return nil
	})
	if err != nil {
		return err
	}

	return d.UploadDelta(buildDir, releaseID, files)
}

// writeFilesToTar writes files to tar writer and returns total size
func writeFilesToTar(tarWriter *tar.Writer, buildDir string, files []string) (int64, error) {
	var totalSize int64
	for _, file := range files {
		fullPath := filepath.Join(buildDir, file)
		if err := addFileToTar(tarWriter, fullPath, file); err != nil {
			return 0, fmt.Errorf("add %s to tar: %w", file, err)
		}
		if info, _ := os.Stat(fullPath); info != nil {
			totalSize += info.Size()
		}
	}
	return totalSize, nil
}

// streamTarXZ creates tar writer and streams XZ-compressed tar to stdin
func streamTarXZ(stdin io.WriteCloser, buildDir string, files []string) (int64, error) {
	xzWriter, err := xz.NewWriter(stdin)
	if err != nil {
		return 0, fmt.Errorf("create xz writer: %w", err)
	}
	tarWriter := tar.NewWriter(xzWriter)
	totalSize, writeErr := writeFilesToTar(tarWriter, buildDir, files)
	tarWriter.Close()
	xzWriter.Close()
	return totalSize, writeErr
}

// UploadDelta uploads only changed files to the release directory via SSH + XZ.
// Uses --unlink-first to break hardlinks before extraction, preserving rollback integrity.
func (d *RemoteDeployer) UploadDelta(buildDir, releaseID string, files []string) error {
	if len(files) == 0 {
		return nil
	}

	// --unlink-first removes existing files before extracting, breaking hardlinks
	// so the original file in the source release remains intact for rollback
	script := fmt.Sprintf("cd '%s' && xz -d | tar --unlink-first -xf -", d.releaseDir(releaseID))
	cmd, stdin, err := d.sshStream(script)
	if err != nil {
		return fmt.Errorf("start ssh: %w", err)
	}

	totalSize, writeErr := streamTarXZ(stdin, buildDir, files)
	stdin.Close()

	if writeErr != nil {
		cmd.Wait()
		return writeErr
	}
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("upload failed: %w", err)
	}

	fmt.Printf("    Uploaded %d files (%.2f MB uncompressed)\n",
		len(files), float64(totalSize)/(1024*1024))
	return nil
}

// addFileToTar adds a file to the tar archive.
func addFileToTar(tw *tar.Writer, fullPath, relPath string) error {
	f, err := os.Open(fullPath)
	if err != nil {
		return err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return err
	}

	header := &tar.Header{
		Name:    relPath,
		Size:    stat.Size(),
		Mode:    int64(stat.Mode()),
		ModTime: stat.ModTime(),
	}

	if err := tw.WriteHeader(header); err != nil {
		return err
	}

	_, err = io.Copy(tw, f)
	return err
}

// Activate validates and activates the release via symlink swap.
func (d *RemoteDeployer) Activate(releaseID string) error {
	releaseDir := d.releaseDir(releaseID)
	currentLink := d.currentLink()

	script := fmt.Sprintf(`
		set -e

		# Validate required files
		for f in healthz.json index.html sw.js; do
			if [ ! -f '%s/'$f ]; then
				echo "ERROR: $f missing"
				rm -rf '%s'
				exit 1
			fi
		done

		# Atomic symlink swap
		ln -sfn '%s' '%s.new'
		mv -Tf '%s.new' '%s'
	`, releaseDir, releaseDir, releaseDir, currentLink, currentLink, currentLink)

	output, err := d.ssh(script)
	if err != nil {
		return fmt.Errorf("activate: %s: %w", output, err)
	}

	return nil
}

// Cleanup removes old releases, keeping the specified number.
func (d *RemoteDeployer) Cleanup(keepN int) error {
	script := fmt.Sprintf(`
		cd '%s' && ls -1t | tail -n +%d | xargs -r rm -rf
	`, d.releasesDir(), keepN+1)

	output, err := d.ssh(script)
	if err != nil {
		return fmt.Errorf("cleanup: %s: %w", output, err)
	}

	return nil
}

// HealthCheck verifies the deployment was successful.
func (d *RemoteDeployer) HealthCheck(releaseID string) error {
	script := fmt.Sprintf(
		"curl -sf http://localhost/healthz.json | grep -q '%s'",
		releaseID,
	)

	_, err := d.ssh(script)
	if err != nil {
		return fmt.Errorf("health check failed: release %s not live", releaseID)
	}

	return nil
}

// ListReleases returns all available releases.
func (d *RemoteDeployer) ListReleases() ([]Release, error) {
	script := fmt.Sprintf(`
		cd '%s' 2>/dev/null || exit 0
		current=$(readlink -f '%s' 2>/dev/null || echo "")
		for dir in */; do
			dir="${dir%%/}"
			[ -d "$dir" ] || continue
			mtime=$(stat -c '%%Y' "$dir" 2>/dev/null || echo "0")
			is_current="false"
			[ "%s/$dir" = "$current" ] && is_current="true"
			echo "$dir $mtime $is_current"
		done
	`, d.releasesDir(), d.currentLink(), d.releasesDir())

	output, err := d.ssh(script)
	if err != nil {
		return nil, fmt.Errorf("list releases: %w", err)
	}

	var releases []Release
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) < 3 {
			continue
		}

		var mtime int64
		fmt.Sscanf(parts[1], "%d", &mtime)

		releases = append(releases, Release{
			ID:        parts[0],
			Path:      filepath.Join(d.releasesDir(), parts[0]),
			CreatedAt: time.Unix(mtime, 0),
			Current:   parts[2] == "true",
		})
	}

	// Sort by creation time (newest first)
	sort.Slice(releases, func(i, j int) bool {
		return releases[i].CreatedAt.After(releases[j].CreatedAt)
	})

	return releases, nil
}

// findPreviousReleaseID finds the first non-current release ID
func (d *RemoteDeployer) findPreviousReleaseID() (string, error) {
	releases, err := d.ListReleases()
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

// activateRemoteRelease activates a release on the remote server
func (d *RemoteDeployer) activateRemoteRelease(releaseID string) error {
	releaseDir := d.releaseDir(releaseID)
	currentLink := d.currentLink()

	script := fmt.Sprintf(`
		set -e
		if [ ! -d '%s' ]; then
			echo "ERROR: release %s not found"
			exit 1
		fi
		ln -sfn '%s' '%s.new'
		mv -Tf '%s.new' '%s'
	`, releaseDir, releaseID, releaseDir, currentLink, currentLink, currentLink)

	output, err := d.ssh(script)
	if err != nil {
		return fmt.Errorf("rollback: %s: %w", output, err)
	}
	return nil
}

// Rollback switches to a previous release.
func (d *RemoteDeployer) Rollback(releaseID string) error {
	targetID := releaseID
	if targetID == "" {
		var err error
		targetID, err = d.findPreviousReleaseID()
		if err != nil {
			return err
		}
	}
	return d.activateRemoteRelease(targetID)
}

// GetCurrentRelease returns the currently active release ID.
func (d *RemoteDeployer) GetCurrentRelease() (string, error) {
	script := fmt.Sprintf("basename $(readlink -f '%s' 2>/dev/null) 2>/dev/null || echo ''", d.currentLink())
	output, err := d.ssh(script)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(output)), nil
}

// GetHealthz returns the current healthz.json content.
func (d *RemoteDeployer) GetHealthz() ([]byte, error) {
	output, err := d.ssh("curl -sf http://localhost/healthz.json")
	if err != nil {
		return nil, err
	}

	// Pretty print JSON
	var buf bytes.Buffer
	if err := json.Indent(&buf, output, "", "  "); err != nil {
		return output, nil
	}

	return buf.Bytes(), nil
}
