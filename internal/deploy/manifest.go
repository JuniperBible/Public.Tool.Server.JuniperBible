package deploy

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"sync"
	"time"
)

// GenerateManifest creates a build manifest for the given directory.
// Files are hashed in parallel using all available CPU cores.
func GenerateManifest(dir string, releaseID string) (*Manifest, error) {
	return GenerateManifestWithWorkers(dir, releaseID, runtime.NumCPU())
}

// collectFiles walks directory and returns list of relative file paths
func collectFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		relPath, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if relPath != "build-manifest.json" {
			files = append(files, relPath)
		}
		return nil
	})
	return files, err
}

// hashWorker processes files from channel and adds to manifest
func hashWorker(dir string, fileChan <-chan string, manifest *Manifest, mu *sync.Mutex, errChan chan<- error, wg *sync.WaitGroup) {
	defer wg.Done()
	for relPath := range fileChan {
		fullPath := filepath.Join(dir, relPath)
		info, err := hashFile(fullPath)
		if err != nil {
			select {
			case errChan <- err:
			default:
			}
			continue
		}
		mu.Lock()
		manifest.Files[relPath] = info
		mu.Unlock()
	}
}

// GenerateManifestWithWorkers creates a build manifest using the specified number of workers.
func GenerateManifestWithWorkers(dir string, releaseID string, workers int) (*Manifest, error) {
	manifest := &Manifest{
		Files:     make(map[string]FileInfo),
		ReleaseID: releaseID,
		BuildTime: time.Now().UTC(),
	}

	files, err := collectFiles(dir)
	if err != nil {
		return nil, err
	}

	var mu sync.Mutex
	fileChan := make(chan string, len(files))
	errChan := make(chan error, 1)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go hashWorker(dir, fileChan, manifest, &mu, errChan, &wg)
	}

	for _, f := range files {
		fileChan <- f
	}
	close(fileChan)
	wg.Wait()

	select {
	case err := <-errChan:
		return nil, err
	default:
	}

	return manifest, nil
}

// hashFile computes the SHA256 hash and size of a file.
func hashFile(path string) (FileInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return FileInfo{}, err
	}
	defer f.Close()

	stat, err := f.Stat()
	if err != nil {
		return FileInfo{}, err
	}

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return FileInfo{}, err
	}

	return FileInfo{
		SHA256: hex.EncodeToString(h.Sum(nil)),
		Size:   stat.Size(),
	}, nil
}

// WriteManifest writes the manifest to a JSON file.
func WriteManifest(m *Manifest, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	return enc.Encode(m)
}

// ReadManifest reads a manifest from a JSON file.
func ReadManifest(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	return &m, nil
}

// findChangedFiles finds files that are new or changed in local manifest
func findChangedFiles(local, remote *Manifest) (changed, unchanged []string) {
	for path, info := range local.Files {
		remoteInfo, exists := remote.Files[path]
		if !exists || remoteInfo.SHA256 != info.SHA256 {
			changed = append(changed, path)
		} else {
			unchanged = append(unchanged, path)
		}
	}
	return
}

// findDeletedFiles finds files that exist in remote but not in local
func findDeletedFiles(local, remote *Manifest) []string {
	var deleted []string
	for path := range remote.Files {
		if _, exists := local.Files[path]; !exists {
			deleted = append(deleted, path)
		}
	}
	return deleted
}

// CalculateDelta compares local and remote manifests to find changed files.
func CalculateDelta(local, remote *Manifest) *Delta {
	changed, unchanged := findChangedFiles(local, remote)
	deleted := findDeletedFiles(local, remote)

	sort.Strings(changed)
	sort.Strings(unchanged)
	sort.Strings(deleted)

	return &Delta{Changed: changed, Unchanged: unchanged, Deleted: deleted}
}

// TotalSize returns the total size of files in the manifest.
func (m *Manifest) TotalSize() int64 {
	var total int64
	for _, info := range m.Files {
		total += info.Size
	}
	return total
}

// DeltaSize returns the total size of changed files.
func DeltaSize(m *Manifest, changed []string) int64 {
	var total int64
	for _, path := range changed {
		if info, ok := m.Files[path]; ok {
			total += info.Size
		}
	}
	return total
}
