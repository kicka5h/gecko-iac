// Package state provides state backend implementations for Gecko.
// The local backend stores state in JSON files under ~/.gecko/state/.
// Remote backends (S3, GCS, etcd) can be implemented by satisfying core.StateBackend.
package state

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
)

// LocalBackend persists state to the local filesystem
type LocalBackend struct {
	baseDir string
	mu      sync.Mutex
	locks   map[string]string // key -> lockID
}

// NewLocalBackend creates a state backend rooted at baseDir
func NewLocalBackend(baseDir string) (*LocalBackend, error) {
	if err := os.MkdirAll(baseDir, 0700); err != nil {
		return nil, fmt.Errorf("failed to create state dir %s: %w", baseDir, err)
	}
	return &LocalBackend{
		baseDir: baseDir,
		locks:   make(map[string]string),
	}, nil
}

// DefaultLocalBackend returns a backend at ~/.gecko/state/
func DefaultLocalBackend() (*LocalBackend, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(home, ".gecko", "state")
	return NewLocalBackend(dir)
}

func (b *LocalBackend) stateFile(stackName, workspace string) string {
	return filepath.Join(b.baseDir, fmt.Sprintf("%s.%s.json", stackName, workspace))
}

func (b *LocalBackend) lockKey(stackName, workspace string) string {
	return stackName + "." + workspace
}

// Load reads state from disk
func (b *LocalBackend) Load(_ context.Context, stackName, workspace string) (*core.StackState, error) {
	path := b.stateFile(stackName, workspace)
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &core.StackState{
			StackName: stackName,
			Workspace: workspace,
			Resources: make(map[core.ResourceID]*core.ResourceState),
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to read state file %s: %w", path, err)
	}
	var state core.StackState
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("failed to parse state file %s: %w", path, err)
	}
	if state.Resources == nil {
		state.Resources = make(map[core.ResourceID]*core.ResourceState)
	}
	return &state, nil
}

// Save writes state atomically to disk
func (b *LocalBackend) Save(_ context.Context, stackName, workspace string, state *core.StackState) error {
	state.UpdatedAt = time.Now()
	state.Version++

	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal state: %w", err)
	}

	path := b.stateFile(stackName, workspace)
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		return fmt.Errorf("failed to write temp state file: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("failed to finalize state file: %w", err)
	}
	return nil
}

// Delete removes all state for a stack+workspace
func (b *LocalBackend) Delete(_ context.Context, stackName, workspace string) error {
	path := b.stateFile(stackName, workspace)
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// List returns all known stack+workspace pairs
func (b *LocalBackend) List(_ context.Context) ([]string, error) {
	entries, err := os.ReadDir(b.baseDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var result []string
	for _, e := range entries {
		if !e.IsDir() && filepath.Ext(e.Name()) == ".json" && filepath.Ext(e.Name()) != ".tmp" {
			result = append(result, e.Name()[:len(e.Name())-5]) // strip .json
		}
	}
	return result, nil
}

// Lock acquires an advisory lock
func (b *LocalBackend) Lock(_ context.Context, stackName, workspace string) (string, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := b.lockKey(stackName, workspace)
	if existing, locked := b.locks[key]; locked {
		return "", fmt.Errorf("state is locked (lockID=%s). Another gecko process may be running", existing)
	}

	lockID := fmt.Sprintf("gecko-%d", time.Now().UnixNano())
	b.locks[key] = lockID

	// Write lock file to disk for cross-process locking
	lockPath := b.stateFile(stackName, workspace) + ".lock"
	lockData := fmt.Sprintf(`{"lockID":%q,"acquired":%q}`, lockID, time.Now().Format(time.RFC3339))
	_ = os.WriteFile(lockPath, []byte(lockData), 0600)

	return lockID, nil
}

// Unlock releases an advisory lock
func (b *LocalBackend) Unlock(_ context.Context, stackName, workspace, lockID string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	key := b.lockKey(stackName, workspace)
	existing, ok := b.locks[key]
	if !ok {
		return nil
	}
	if existing != lockID {
		return fmt.Errorf("cannot unlock: lockID mismatch")
	}

	delete(b.locks, key)
	lockPath := b.stateFile(stackName, workspace) + ".lock"
	_ = os.Remove(lockPath)
	return nil
}

// GetResourceCount returns the number of managed resources
func (b *LocalBackend) GetResourceCount(ctx context.Context, stackName, workspace string) (int, error) {
	state, err := b.Load(ctx, stackName, workspace)
	if err != nil {
		return 0, err
	}
	return len(state.Resources), nil
}
