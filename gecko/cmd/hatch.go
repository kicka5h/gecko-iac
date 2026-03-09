package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/config"
	"github.com/gecko-iac/gecko/internal/ui"
)

var hatchCmd = &Command{
	Name:    "hatch",
	Short:   "Initialize a new Gecko project",
	Aliases: []string{"init"},
	Long: `gecko hatch initializes a new infrastructure project in the current directory.
Like a gecko hatching from an egg, this is where your infrastructure is born.

Creates gecko.json, example stack files, and provider configurations based
on your selected FOSS infrastructure stack.`,
	Args: []string{"project-name"},
	Flags: []Flag{
		{Name: "runtime", Short: "r", Default: "scute", Usage: "Stack runtime: go, python, typescript, hcl"},
		{Name: "providers", Short: "p", Default: "k8s", Usage: "Comma-separated providers: k8s, proxmox, nomad, gitea, minio, vault"},
		{Name: "workspace", Short: "w", Default: "dev", Usage: "Initial workspace name"},
		{Name: "backend", Short: "b", Default: "local", Usage: "State backend: local, s3, gcs, etcd, postgres"},
		{Name: "force", Short: "f", Default: "false", Usage: "Overwrite existing gecko.json"},
		{Name: "template", Short: "t", Default: "", Usage: "Bootstrap from a template: k8s-homelab, proxmox-cluster, gitops"},
	},
	Run: runHatch,
}

func runHatch(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("hatch")

	// Resolve project name
	projectName := ""
	if len(args) > 0 {
		projectName = args[0]
	} else {
		wd, _ := os.Getwd()
		projectName = filepath.Base(wd)
	}

	projectName = strings.ToLower(strings.ReplaceAll(projectName, " ", "-"))
	runtime := flagVal(flags, "runtime", "scute")
	providers := strings.Split(flagVal(flags, "providers", "k8s"), ",")
	workspace := flagVal(flags, "workspace", "dev")
	backend := flagVal(flags, "backend", "local")
	template := flagVal(flags, "template", "")

	// Check for existing project
	if _, err := os.Stat("gecko.json"); err == nil && !flagSet(flags, "force") {
		ui.Warn("gecko.json already exists. Use --force to reinitialize.")
		return nil
	}

	ui.Info(fmt.Sprintf("Hatching project %q...", projectName))
	fmt.Println()

	// Display configuration
	ui.Header("Project Configuration")
	ui.Label("Name", projectName)
	ui.Label("Runtime", runtime)
	ui.Label("Providers", strings.Join(providers, ", "))
	ui.Label("Workspace", workspace)
	ui.Label("State Backend", backend)
	if template != "" {
		ui.Label("Template", template)
	}
	fmt.Println()

	// Build provider configs
	providerConfigs := make(map[string]config.ProviderConfig)
	for _, p := range providers {
		p = strings.TrimSpace(p)
		providerConfigs[p] = defaultProviderConfig(p)
	}

	cfg := &config.ProjectConfig{
		Name:       projectName,
		Runtime:    runtime,
		Workspace:  workspace,
		Workspaces: []string{"dev", "staging", "prod"},
		Providers:  providerConfigs,
		Backend: config.BackendConfig{
			Type:   backend,
			Config: defaultBackendConfig(backend),
		},
		Tags: map[string]string{
			"managed-by": "gecko",
			"project":    projectName,
			"created-at": time.Now().Format("2006-01-02"),
		},
	}

	// Step 1: Write gecko.json
	spin := ui.NewSpinner("Writing gecko.json")
	spin.Start()
	time.Sleep(400 * time.Millisecond)
	if err := config.SaveProject(".", cfg); err != nil {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Failed to write gecko.json: %v", err))
		return err
	}
	spin.Stop(true)

	// Step 2: Create stack directory
	spin = ui.NewSpinner("Creating stack layout")
	spin.Start()
	time.Sleep(350 * time.Millisecond)
	if err := createProjectLayout(projectName, runtime, providers, workspace); err != nil {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Failed to create project layout: %v", err))
		return err
	}
	spin.Stop(true)

	// Step 3: Create .geckoignore
	spin = ui.NewSpinner("Writing .geckoignore")
	spin.Start()
	time.Sleep(200 * time.Millisecond)
	writeGeckoIgnore()
	spin.Stop(true)

	// Step 4: Initialize workspace state
	spin = ui.NewSpinner(fmt.Sprintf("Initializing workspace %q", workspace))
	spin.Start()
	time.Sleep(300 * time.Millisecond)
	os.MkdirAll(filepath.Join(".gecko", "state"), 0700)
	spin.Stop(true)

	fmt.Println()
	ui.Divider()

	if template != "" {
		applyTemplate(template, projectName, providers)
	}

	fmt.Printf("\n  %s🥚 Project hatched!%s Your gecko infrastructure is ready.\n\n", ui.GeckoGreen+ui.Bold, ui.Reset)

	// Next steps
	ui.Header("Next Steps")
	printStep(1, "Explore your stack:", fmt.Sprintf("cat stacks/%s/main.go", workspace))
	printStep(2, "Preview your infrastructure:", "gecko crawl")
	printStep(3, "Apply your infrastructure:", "gecko grip")
	printStep(4, "Check status:", "gecko bask")
	fmt.Println()

	return nil
}

func printStep(n int, label, cmd string) {
	fmt.Printf("  %s%d.%s %s%s%s\n", ui.GeckoTeal+ui.Bold, n, ui.Reset, ui.BrightWhite, label, ui.Reset)
	fmt.Printf("     %sgecko %s%s\n", ui.GeckoMuted, cmd, ui.Reset)
}

func createProjectLayout(name, runtime string, providers []string, workspace string) error {
	dirs := []string{
		"stacks/" + workspace,
		"stacks/staging",
		"stacks/prod",
		"modules",
		".gecko/state",
		".gecko/plans",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(d, 0755); err != nil {
			return err
		}
	}

	// Write example stack file based on runtime
	stackContent := generateStackFile(name, runtime, providers, workspace)
	mainFile := mainFileName(runtime)
	return os.WriteFile(filepath.Join("stacks", workspace, mainFile), []byte(stackContent), 0644)
}

func mainFileName(runtime string) string {
	switch runtime {
	case "python":
		return "__main__.py"
	case "typescript":
		return "index.ts"
	case "hcl":
		return "main.hcl"
	case "go":
		return "main.go"
	default: // scute
		return "main.scute"
	}
}

func generateStackFile(name, runtime string, providers []string, workspace string) string {
	switch runtime {
	case "scute", "":
		return generateScuteStack(name, providers, workspace)
	case "python":
		return fmt.Sprintf(`import gecko

stack = gecko.Stack(name=%q, workspace=%q)

# Configure providers
%s
# Define your resources here
# Example: k8s namespace
# ns = gecko.Resource(stack, "k8s:namespace", name="app-namespace",
#     inputs={"name": "my-app", "labels": {"env": workspace}})

gecko.export(stack)
`, name, workspace, pythonProviders(providers))

	case "typescript":
		return fmt.Sprintf(`import * as gecko from "@gecko-iac/gecko";

const stack = new gecko.Stack({ name: %q, workspace: %q });

// Configure providers
%s
// Define your resources here
// Example:
// const ns = new gecko.Resource(stack, "k8s:namespace", {
//   name: "app-namespace",
//   inputs: { name: "my-app" }
// });

export default stack;
`, name, workspace, tsProviders(providers))

	default: // Go
		return fmt.Sprintf(`package main

import (
	"context"

	"github.com/gecko-iac/gecko/core"
	%s
)

func main() {
	ctx := context.Background()

	// Create your stack
	stack := core.NewStack(%q, "main", %q)

	// Register providers
	%s
	// ─── Define Resources ─────────────────────────────────────────────────────
	// Declare your infrastructure below. Gecko computes a dependency graph
	// automatically and applies changes in the correct order.
	//
	// Example: Kubernetes namespace
	// nsID := stack.Resource(core.ResourceArgs{
	// 	Type: "k8s:namespace",
	// 	Name: "app-namespace",
	// 	Inputs: core.Inputs{
	// 		"name": "my-app",
	// 		"labels": map[string]string{"env": %q},
	// 	},
	// })
	//
	// Example: Deployment depending on namespace
	// _ = stack.Resource(core.ResourceArgs{
	// 	Type:      "k8s:deployment",
	// 	Name:      "api-server",
	// 	DependsOn: []core.ResourceID{nsID},
	// 	Inputs: core.Inputs{
	// 		"namespace": "my-app",
	// 		"image":     "nginx:latest",
	// 		"replicas":  3,
	// 	},
	// })
	_ = ctx
	_ = stack
}
`, goImports(providers), name, workspace, goProviders(providers), workspace)
	}
}

func goImports(providers []string) string {
	var imports []string
	for _, p := range providers {
		switch strings.TrimSpace(p) {
		case "k8s", "kubernetes":
			imports = append(imports, `"github.com/gecko-iac/gecko/providers/k8s"`)
		case "proxmox":
			imports = append(imports, `"github.com/gecko-iac/gecko/providers/proxmox"`)
		case "nomad":
			imports = append(imports, `"github.com/gecko-iac/gecko/providers/nomad"`)
		}
	}
	if len(imports) == 0 {
		return ""
	}
	return strings.Join(imports, "\n\t")
}

func goProviders(providers []string) string {
	var lines []string
	for _, p := range providers {
		p = strings.TrimSpace(p)
		lines = append(lines, fmt.Sprintf(`stack.RegisterProvider(%s.NewProvider(nil))`, p))
	}
	return strings.Join(lines, "\n\t")
}

func pythonProviders(providers []string) string {
	var lines []string
	for _, p := range providers {
		p = strings.TrimSpace(p)
		lines = append(lines, fmt.Sprintf(`stack.register_provider(gecko.providers.%s())`, p))
	}
	return strings.Join(lines, "\n") + "\n\n"
}

func tsProviders(providers []string) string {
	var lines []string
	for _, p := range providers {
		p = strings.TrimSpace(p)
		lines = append(lines, fmt.Sprintf(`stack.registerProvider(new gecko.providers.%s.Provider());`, p))
	}
	return strings.Join(lines, "\n") + "\n\n"
}

func defaultProviderConfig(provider string) config.ProviderConfig {
	switch provider {
	case "k8s", "kubernetes":
		return config.ProviderConfig{
			Type:    "kubernetes",
			Version: ">=1.28",
			Config:  map[string]interface{}{"kubeconfig": "~/.kube/config"},
		}
	case "proxmox":
		return config.ProviderConfig{
			Type:    "proxmox",
			Version: ">=8.0",
			Config:  map[string]interface{}{"endpoint": "https://proxmox.local:8006", "insecure": false},
		}
	case "nomad":
		return config.ProviderConfig{
			Type:    "nomad",
			Version: ">=1.7",
			Config:  map[string]interface{}{"address": "http://nomad.service.consul:4646"},
		}
	case "gitea":
		return config.ProviderConfig{
			Type:    "gitea",
			Version: ">=1.21",
			Config:  map[string]interface{}{"server_url": "https://gitea.local"},
		}
	case "minio":
		return config.ProviderConfig{
			Type:   "minio",
			Config: map[string]interface{}{"endpoint": "minio.local:9000", "use_ssl": true},
		}
	case "vault":
		return config.ProviderConfig{
			Type:   "vault",
			Config: map[string]interface{}{"address": "https://vault.local:8200"},
		}
	default:
		return config.ProviderConfig{Type: provider}
	}
}

func defaultBackendConfig(backend string) map[string]interface{} {
	switch backend {
	case "s3":
		return map[string]interface{}{
			"bucket": "gecko-state",
			"region": "us-east-1",
			"prefix": "gecko/state/",
		}
	case "gcs":
		return map[string]interface{}{
			"bucket": "gecko-state",
			"prefix": "gecko/state/",
		}
	case "etcd":
		return map[string]interface{}{
			"endpoints": []string{"http://etcd:2379"},
			"prefix":    "/gecko/state/",
		}
	case "postgres":
		return map[string]interface{}{
			"dsn":   "postgres://gecko:gecko@localhost/geckostate",
			"table": "gecko_state",
		}
	default:
		return nil
	}
}

func writeGeckoIgnore() {
	content := `# Gecko ignore file
# Files and directories to exclude from gecko operations

# State files (managed by gecko)
.gecko/state/

# Local development overrides
*.local.json
*.dev.json

# Secrets
*.secret
.env
.env.*

# Build artifacts
dist/
build/
__pycache__/
node_modules/
`
	_ = os.WriteFile(".geckoignore", []byte(content), 0644)
}

func applyTemplate(template, name string, providers []string) {
	switch template {
	case "k8s-homelab":
		ui.Header("Applying k8s-homelab template")
		ui.Indent("✓ cert-manager resources")
		ui.Indent("✓ ingress-nginx namespace")
		ui.Indent("✓ metallb configuration")
		ui.Indent("✓ longhorn storage class")
	case "proxmox-cluster":
		ui.Header("Applying proxmox-cluster template")
		ui.Indent("✓ VM pool configuration")
		ui.Indent("✓ network bridge setup")
		ui.Indent("✓ storage pool definitions")
	case "gitops":
		ui.Header("Applying gitops template")
		ui.Indent("✓ Gitea repository")
		ui.Indent("✓ ArgoCD application")
		ui.Indent("✓ Webhook configuration")
	default:
		ui.Warn(fmt.Sprintf("Unknown template: %q. Skipping.", template))
	}
	fmt.Println()
}


func generateScuteStack(name string, providers []string, workspace string) string {
	var habitats strings.Builder
	var spawns strings.Builder

	for _, p := range providers {
		p = strings.TrimSpace(p)
		switch p {
		case "k8s", "kubernetes":
			habitats.WriteString(`habitat "k8s"
  kubeconfig: env("KUBECONFIG") | "~/.kube/config"
  context:    env("KUBE_CONTEXT") | "homelab"
end

`)
			spawns.WriteString(`# ─── Namespaces ──────────────────────────────────────────────────────────
spawn "k8s:namespace" as "app-ns"
  name:   app_name
  labels: common_tags
end

# ─── Deployment ───────────────────────────────────────────────────────────
spawn "k8s:deployment" as "api"
  needs:     @app-ns
  namespace: @app-ns.name
  image:     "` + "`" + `#{app_name}:#{image_tag}` + "`" + `"
  replicas:  replicas

  env:
    APP_ENV:   workspace
    LOG_LEVEL: log_level
  end

  resources:
    requests:
      cpu:    "100m"
      memory: "128Mi"
    end
    limits:
      cpu:    "500m"
      memory: "512Mi"
    end
  end

  when is_production
    replicas: replicas * 3
  end
end

# ─── Service ──────────────────────────────────────────────────────────────
spawn "k8s:service" as "api-svc"
  needs:       @api
  namespace:   @app-ns.name
  port:        80
  target_port: 8080
end

# ─── Team namespaces (one per team via across) ────────────────────────────
spawn "k8s:namespace" as "team-ns" across ["backend", "frontend", "data"]
  name:   item
  labels:
    team:       item
    managed-by: "gecko"
  end
end

`)
		case "proxmox":
			habitats.WriteString(`habitat "proxmox"
  endpoint: env("PROXMOX_ENDPOINT") | "https://proxmox.local:8006"
  token:    secret("proxmox.api.token")
end

`)
			spawns.WriteString(`# ─── VMs (one per worker index via across) ───────────────────────────────
spawn "proxmox:vm" as "worker" across 1..worker_count
  node:   "pve"
  name:   "worker-#{item}"
  cores:  2
  memory: 4096
  clone:  "ubuntu-template"
  tags:   [workspace, "kubernetes", "worker"]
end

`)
		case "gitea":
			habitats.WriteString(`habitat "gitea"
  server_url: env("GITEA_URL") | "https://gitea.local"
  token:      secret("gitea.admin.token")
end

`)
			spawns.WriteString(`# ─── Infra Repo ──────────────────────────────────────────────────────────
spawn "gitea:repo" as "infra-repo"
  owner:     "platform"
  name:      "` + name + `-infra"
  private:   true
  auto_init: true
end

# ─── Deploy webhook ───────────────────────────────────────────────────────
spawn "gitea:webhook" as "deploy-hook"
  needs:  @infra-repo
  owner:  @infra-repo.owner
  repo:   @infra-repo.name
  url:    "https://ci.#{base_domain}/hooks/gitea"
  events: ["push", "pull_request"]
  active: true
end

`)
		case "minio":
			habitats.WriteString(`habitat "minio"
  endpoint:   env("MINIO_ENDPOINT") | "minio.local:9000"
  access_key: secret("minio.access.key")
  secret_key: secret("minio.secret.key")
  ssl:        true
end

`)
			spawns.WriteString(`# ─── State Bucket ────────────────────────────────────────────────────────
spawn "minio:bucket" as "state-bucket"
  name:       "#{app_name}-state-#{workspace}"
  versioning: true
  policy:     "private"
end

`)
		}
	}

	return fmt.Sprintf(`# %s — Scute Infrastructure Stack
# Workspace: %s
#
# Scute uses gecko-themed vocabulary:
#   territory → project    habitat  → provider    spawn → resource
#   mark      → variable   camouflage → locals    signal → output
#   @name     → reference  item     → loop var    |      → fallback
#
# gecko crawl   preview changes
# gecko grip    apply changes
# gecko bask    status dashboard
# gecko run     dry-evaluate this file

# ─── Territory ────────────────────────────────────────────────────────────────
territory "%s"
  workspace: env("GECKO_WORKSPACE") | "%s"
end

# ─── Habitats (providers) ─────────────────────────────────────────────────────
%s
# ─── Marks (variables) ────────────────────────────────────────────────────────
# Override any mark at apply-time: GECKO_VAR_REPLICAS=5 gecko grip

mark app_name     string: "%s"
mark workspace    string: "%s"
mark image_tag    string: "latest"
mark replicas     number: 2
mark log_level    string: "info"
mark base_domain  string: "cluster.local"
mark worker_count number: 3

# ─── Camouflage (computed locals) ─────────────────────────────────────────────
camouflage
  is_production: workspace == "prod"
  is_staging:    workspace == "staging"
  name_prefix:   "#{app_name}-#{workspace}"
  common_tags:
    app:        app_name
    env:        workspace
    managed-by: "gecko"
  end
end

# ─── Spawns (resources) ───────────────────────────────────────────────────────
%s
# ─── Signals (outputs) ────────────────────────────────────────────────────────
signal "workspace"
  value:       workspace
  description: "Active workspace name"
end

signal "app"
  value:       app_name
  description: "Application name"
end
`, name, workspace, name, workspace, habitats.String(), name, workspace, spawns.String())
}
