package lang

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SnackSource describes a resolved snack location
type SnackSource struct {
	Kind     string // "local", "https", "gitea", "registry"
	RawURL   string // normalized URL or path
	CachePath string
}

// FetchSnack resolves a snack source string to raw .snack file content.
// Sources can be:
//
//	./path/to/module.snack         — local file (relative to caller)
//	/abs/path/to/module.snack      — local file (absolute)
//	https://host/path/file.snack   — direct HTTPS URL
//	https://host/path/file@v1.2.0  — versioned HTTPS URL
//	gitea://host/owner/repo/file.snack@ref — Gitea raw file
//	snack:org/name@version         — gecko snack registry (future)
func FetchSnack(source, callerDir, cacheDir string) ([]byte, *SnackSource, error) {
	src := &SnackSource{RawURL: source}

	switch {
	case strings.HasPrefix(source, "./") || strings.HasPrefix(source, "../") || strings.HasPrefix(source, "/"):
		src.Kind = "local"
		return fetchLocal(source, callerDir, src)

	case strings.HasPrefix(source, "https://"):
		src.Kind = "https"
		return fetchHTTPS(source, cacheDir, src)

	case strings.HasPrefix(source, "http://"):
		src.Kind = "https"
		return fetchHTTPS(source, cacheDir, src)

	case strings.HasPrefix(source, "gitea://"):
		src.Kind = "gitea"
		return fetchGitea(source, cacheDir, src)

	case strings.HasPrefix(source, "github://"):
		src.Kind = "github"
		return fetchGitHub(source, cacheDir, src)

	case strings.HasPrefix(source, "snack:"):
		src.Kind = "registry"
		return fetchRegistry(source, cacheDir, src)

	default:
		// Treat as local path without ./ prefix
		src.Kind = "local"
		return fetchLocal(source, callerDir, src)
	}
}

// ─── Local ────────────────────────────────────────────────────────────────────

func fetchLocal(source, callerDir string, src *SnackSource) ([]byte, *SnackSource, error) {
	path := source
	if !filepath.IsAbs(path) {
		path = filepath.Join(callerDir, path)
	}
	// Auto-append .snack if no extension
	if filepath.Ext(path) == "" {
		path += ".snack"
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, src, fmt.Errorf("snack local path %q: %w", source, err)
	}
	src.RawURL = abs
	src.CachePath = abs
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, src, fmt.Errorf("snack local %q not found: %w\n  Tip: check the path relative to your .scute file", source, err)
	}
	return data, src, nil
}

// ─── HTTPS ────────────────────────────────────────────────────────────────────

func fetchHTTPS(source, cacheDir string, src *SnackSource) ([]byte, *SnackSource, error) {
	cacheKey := hashURL(source)
	cachePath := filepath.Join(cacheDir, "snacks", cacheKey+".snack")
	src.CachePath = cachePath

	// Use cache if fresh (24 hours)
	if data, ok := readCache(cachePath, 24*time.Hour); ok {
		return data, src, nil
	}

	data, err := httpGet(source)
	if err != nil {
		// Fall back to stale cache on network failure
		if stale, ok := readCache(cachePath, 0); ok {
			return stale, src, nil
		}
		return nil, src, fmt.Errorf("snack fetch %q failed: %w", source, err)
	}

	if err := writeCache(cachePath, data); err != nil {
		// Non-fatal: proceed even if we can't cache
		_ = err
	}
	return data, src, nil
}

// ─── Gitea ────────────────────────────────────────────────────────────────────

// fetchGitea handles gitea://host/owner/repo/path/file.snack@ref
// Converts to: https://host/owner/repo/raw/branch/path/file.snack
func fetchGitea(source, cacheDir string, src *SnackSource) ([]byte, *SnackSource, error) {
	// Strip gitea:// prefix
	rest := strings.TrimPrefix(source, "gitea://")

	ref := "main"
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		ref = rest[at+1:]
		rest = rest[:at]
	}

	// rest = host/owner/repo/path/to/file.snack
	parts := strings.SplitN(rest, "/", 4)
	if len(parts) < 4 {
		return nil, src, fmt.Errorf("invalid gitea:// source %q — expected gitea://host/owner/repo/path[@ref]", source)
	}
	host, owner, repo, filePath := parts[0], parts[1], parts[2], parts[3]
	rawURL := fmt.Sprintf("https://%s/%s/%s/raw/branch/%s/%s", host, owner, repo, ref, filePath)
	if !strings.HasSuffix(filePath, ".snack") {
		rawURL += ".snack"
	}
	src.RawURL = rawURL
	return fetchHTTPS(rawURL, cacheDir, src)
}

// ─── GitHub ───────────────────────────────────────────────────────────────────

// fetchGitHub handles github://owner/repo/path/file.snack@ref
func fetchGitHub(source, cacheDir string, src *SnackSource) ([]byte, *SnackSource, error) {
	rest := strings.TrimPrefix(source, "github://")

	ref := "main"
	if at := strings.LastIndex(rest, "@"); at >= 0 {
		ref = rest[at+1:]
		rest = rest[:at]
	}

	parts := strings.SplitN(rest, "/", 3)
	if len(parts) < 3 {
		return nil, src, fmt.Errorf("invalid github:// source %q — expected github://owner/repo/path[@ref]", source)
	}
	owner, repo, filePath := parts[0], parts[1], parts[2]
	rawURL := fmt.Sprintf("https://raw.githubusercontent.com/%s/%s/%s/%s", owner, repo, ref, filePath)
	if !strings.HasSuffix(filePath, ".snack") {
		rawURL += ".snack"
	}
	src.RawURL = rawURL
	return fetchHTTPS(rawURL, cacheDir, src)
}

// ─── Registry ─────────────────────────────────────────────────────────────────

// fetchRegistry handles snack:org/name@version
// Resolves via the official Gecko snack registry at snacks.gecko-iac.dev
func fetchRegistry(source, cacheDir string, src *SnackSource) ([]byte, *SnackSource, error) {
	ref := strings.TrimPrefix(source, "snack:")
	version := "latest"
	if at := strings.LastIndex(ref, "@"); at >= 0 {
		version = ref[at+1:]
		ref = ref[:at]
	}
	rawURL := fmt.Sprintf("https://snacks.gecko-iac.dev/v1/%s/%s.snack", ref, version)
	src.RawURL = rawURL
	return fetchHTTPS(rawURL, cacheDir, src)
}

// ─── HTTP Helpers ─────────────────────────────────────────────────────────────

func httpGet(url string) ([]byte, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP %d from %s", resp.StatusCode, url)
	}
	return io.ReadAll(resp.Body)
}

func hashURL(url string) string {
	h := sha256.Sum256([]byte(url))
	return fmt.Sprintf("%x", h[:8])
}

// ─── Cache ────────────────────────────────────────────────────────────────────

func readCache(path string, maxAge time.Duration) ([]byte, bool) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, false
	}
	if maxAge > 0 && time.Since(info.ModTime()) > maxAge {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	return data, true
}

func writeCache(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// CacheDir returns the gecko snack cache directory (~/.gecko/snacks/)
func CacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".gecko", "snacks")
}

// ─── Registry Listing ─────────────────────────────────────────────────────────

// SnackInfo describes a cached snack module
type SnackInfo struct {
	Name      string
	Source    string
	CachePath string
	Size      int64
	CachedAt  time.Time
}

// ListCachedSnacks returns all snacks in the local cache
func ListCachedSnacks(cacheDir string) ([]SnackInfo, error) {
	snackDir := filepath.Join(cacheDir, "snacks")
	entries, err := os.ReadDir(snackDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var infos []SnackInfo
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".snack") {
			continue
		}
		info, _ := e.Info()
		infos = append(infos, SnackInfo{
			Name:      strings.TrimSuffix(e.Name(), ".snack"),
			CachePath: filepath.Join(snackDir, e.Name()),
			Size:      info.Size(),
			CachedAt:  info.ModTime(),
		})
	}
	return infos, nil
}
