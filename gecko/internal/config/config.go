// Package config handles loading and validation of gecko.yaml project files
// and ~/.gecko/config.yaml user configuration.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ProjectConfig is the content of gecko.yaml in a project root
type ProjectConfig struct {
	Name        string                       `json:"name"`
	Description string                       `json:"description,omitempty"`
	Runtime     string                       `json:"runtime"`           // "go", "python", "typescript", "hcl"
	Workspace   string                       `json:"workspace"`         // active workspace
	Workspaces  []string                     `json:"workspaces"`        // all defined workspaces
	Providers   map[string]ProviderConfig    `json:"providers"`
	Backend     BackendConfig                `json:"backend"`
	Tags        map[string]string            `json:"tags,omitempty"`
}

// ProviderConfig is provider-specific configuration
type ProviderConfig struct {
	Type    string                 `json:"type"`
	Version string                 `json:"version,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// BackendConfig describes where state is stored
type BackendConfig struct {
	Type   string                 `json:"type"`   // "local", "s3", "gcs", "etcd", "postgres"
	Config map[string]interface{} `json:"config,omitempty"`
}

// GeckoConfig is the user-level ~/.gecko/config.json
type GeckoConfig struct {
	DefaultWorkspace string            `json:"default_workspace"`
	Telemetry        bool              `json:"telemetry"`
	Color            bool              `json:"color"`
	Aliases          map[string]string `json:"aliases,omitempty"`
}

const projectFile = "gecko.yaml"
const projectFileJSON = "gecko.json"

// LoadProject loads gecko.yaml or gecko.json from the current or parent directories
func LoadProject() (*ProjectConfig, string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return nil, "", err
	}

	for {
		// Try JSON first (our internal format), then YAML stub
		for _, name := range []string{projectFileJSON, projectFile} {
			path := filepath.Join(dir, name)
			if _, err := os.Stat(path); err == nil {
				cfg, err := loadProjectFile(path)
				if err != nil {
					return nil, "", err
				}
				return cfg, dir, nil
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return nil, "", fmt.Errorf("no gecko.json or gecko.yaml found in current or parent directories.\n  Run 'gecko hatch' to initialize a new project")
}

func loadProjectFile(path string) (*ProjectConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", path, err)
	}

	var cfg ProjectConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", path, err)
	}
	return &cfg, nil
}

// SaveProject writes project config to gecko.json in the given directory
func SaveProject(dir string, cfg *ProjectConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(dir, projectFileJSON)
	return os.WriteFile(path, data, 0644)
}

// LoadUserConfig loads ~/.gecko/config.json
func LoadUserConfig() (*GeckoConfig, error) {
	home, _ := os.UserHomeDir()
	path := filepath.Join(home, ".gecko", "config.json")

	cfg := &GeckoConfig{
		DefaultWorkspace: "dev",
		Color:            true,
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return nil, err
	}
	_ = json.Unmarshal(data, cfg)
	return cfg, nil
}

// SaveUserConfig writes ~/.gecko/config.json
func SaveUserConfig(cfg *GeckoConfig) error {
	home, _ := os.UserHomeDir()
	dir := filepath.Join(home, ".gecko")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.json"), data, 0600)
}

// DefaultProjectConfig returns a sensible default for a new project
func DefaultProjectConfig(name string) *ProjectConfig {
	return &ProjectConfig{
		Name:      name,
		Runtime:   "go",
		Workspace: "dev",
		Workspaces: []string{"dev", "staging", "prod"},
		Providers: map[string]ProviderConfig{
			"k8s": {
				Type:    "kubernetes",
				Version: ">=1.28",
				Config: map[string]interface{}{
					"kubeconfig": "~/.kube/config",
				},
			},
		},
		Backend: BackendConfig{
			Type: "local",
		},
		Tags: map[string]string{
			"managed-by": "gecko",
			"project":    name,
		},
	}
}
