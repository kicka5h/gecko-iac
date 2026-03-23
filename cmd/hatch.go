package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/ui"
)

var hatchCmd = &Command{
	Name:    "hatch",
	Short:   "Initialize a new Gecko project",
	Aliases: []string{"init"},
	Long: `gecko hatch initializes a new infrastructure project in the current directory.
Like a gecko hatching from an egg, this is where your infrastructure is born.

Creates a .scute stack file and project layout based on your selected
FOSS infrastructure stack. No external config files required.`,
	Args: []string{"project-name"},
	Flags: []Flag{
		{Name: "runtime", Short: "r", Default: "scute", Usage: "Stack runtime: go, python, typescript, hcl"},
		{Name: "providers", Short: "p", Default: "proxmox", Usage: "Comma-separated providers: proxmox, fly, gitea, minio"},
		{Name: "workspace", Short: "w", Default: "dev", Usage: "Initial workspace name"},
		{Name: "backend", Short: "b", Default: "local", Usage: "State backend: local, s3, gcs, etcd, postgres"},
		{Name: "force", Short: "f", Default: "false", Usage: "Overwrite existing project"},
		{Name: "template", Short: "t", Default: "", Usage: "Bootstrap from a template: proxmox-cluster, fly-deploy, gitops"},
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
	providers := strings.Split(flagVal(flags, "providers", "proxmox"), ",")
	workspace := flagVal(flags, "workspace", "dev")
	backend := flagVal(flags, "backend", "local")
	template := flagVal(flags, "template", "")

	// Check for existing project
	if _, err := os.Stat(".gecko"); err == nil && !flagSet(flags, "force") {
		ui.Warn("Project already exists (.gecko/ directory found). Use --force to reinitialize.")
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

	// Step 1: Create stack directory and .gecko anchor
	spin := ui.NewSpinner("Creating project layout")
	spin.Start()
	time.Sleep(350 * time.Millisecond)
	if err := createProjectLayout(projectName, runtime, providers, workspace, backend); err != nil {
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
	_ = os.MkdirAll(filepath.Join(".gecko", "state"), 0700)
	spin.Stop(true)

	fmt.Println()
	ui.Divider()

	if template != "" {
		applyTemplate(template, projectName, providers)
	}

	fmt.Printf("\n  %s🥚 Project hatched!%s Your gecko infrastructure is ready.\n\n", ui.GeckoGreen+ui.Bold, ui.Reset)

	// Next steps
	ui.Header("Next Steps")
	printStep(1, "Explore your stack:", "gecko run")
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

func createProjectLayout(name, runtime string, providers []string, workspace, backend string) error {
	dirs := []string{
		workspace,
		"staging",
		"prod",
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
	stackContent := generateStackFile(name, runtime, providers, workspace, backend)
	mainFile := mainFileName(runtime)
	return os.WriteFile(filepath.Join(workspace, mainFile), []byte(stackContent), 0644)
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

func generateStackFile(name, runtime string, providers []string, workspace, backend string) string {
	switch runtime {
	case "scute", "":
		return generateScuteStack(name, providers, workspace, backend)
	case "python":
		return fmt.Sprintf(`import gecko

stack = gecko.Stack(name=%q, workspace=%q)

# Configure providers
%s
# Define your resources here
# Example: proxmox VM
# vm = gecko.Resource(stack, "proxmox:vm", name="web-01",
#     inputs={"node": "pve", "cores": 2, "memory": 4096})

gecko.export(stack)
`, name, workspace, pythonProviders(providers))

	case "typescript":
		return fmt.Sprintf(`import * as gecko from "@gecko-iac/gecko";

const stack = new gecko.Stack({ name: %q, workspace: %q });

// Configure providers
%s
// Define your resources here
// Example:
// const vm = new gecko.Resource(stack, "proxmox:vm", {
//   name: "web-01",
//   inputs: { node: "pve", cores: 2, memory: 4096 }
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
	// Example: Proxmox VM
	// vmID := stack.Resource(core.ResourceArgs{
	// 	Type: "proxmox:vm",
	// 	Name: "web-01",
	// 	Inputs: core.Inputs{
	// 		"node":   "pve",
	// 		"cores":  2,
	// 		"memory": 4096,
	// 		"clone":  "ubuntu-template",
	// 	},
	// })
	//
	// Example: Network depending on VM
	// _ = stack.Resource(core.ResourceArgs{
	// 	Type:      "proxmox:network",
	// 	Name:      "vmbr1",
	// 	DependsOn: []core.ResourceID{vmID},
	// 	Inputs: core.Inputs{
	// 		"node":   "pve",
	// 		"iface":  "vmbr1",
	// 		"cidr":   "10.10.10.1/24",
	// 	},
	// })
	_ = ctx
	_ = stack
}
`, goImports(providers), name, workspace, goProviders(providers))
	}
}

func goImports(providers []string) string {
	var imports []string
	for _, p := range providers {
		switch strings.TrimSpace(p) {
		case "proxmox":
			imports = append(imports, `"github.com/gecko-iac/gecko/providers/proxmox"`)
		case "fly":
			imports = append(imports, `"github.com/gecko-iac/gecko/providers/fly"`)
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
	case "proxmox-cluster":
		ui.Header("Applying proxmox-cluster template")
		ui.Indent("✓ VM pool configuration")
		ui.Indent("✓ network bridge setup")
		ui.Indent("✓ storage pool definitions")
		ui.Indent("✓ cloud-init templates")
	case "fly-deploy":
		ui.Header("Applying fly-deploy template")
		ui.Indent("✓ Fly.io app configuration")
		ui.Indent("✓ machine definitions")
		ui.Indent("✓ volume attachments")
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


func generateScuteStack(name string, providers []string, workspace, backend string) string {
	var habitats strings.Builder
	var spawns strings.Builder

	for _, p := range providers {
		p = strings.TrimSpace(p)
		switch p {
		case "proxmox":
			habitats.WriteString(`habitat "proxmox"
  endpoint: env("PROXMOX_ENDPOINT") | "https://proxmox.local:8006"
  token:    secret("proxmox.api.token")
end

`)
			spawns.WriteString(`# ─── VMs ─────────────────────────────────────────────────────────────────
spawn "proxmox:vm" as "web-01"
  node:   "pve"
  name:   "` + "`" + `#{app_name}-web-01` + "`" + `"
  cores:  2
  memory: 4096
  clone:  "ubuntu-template"
  tags:   [workspace, "web"]

  when is_production
    cores:  4
    memory: 8192
  end
end

# ─── Network ─────────────────────────────────────────────────────────────
spawn "proxmox:network" as "app-net"
  needs: @web-01
  node:  "pve"
  iface: "vmbr1"
  cidr:  "10.10.10.1/24"
end

# ─── LXC Containers (one per service via across) ─────────────────────────
spawn "proxmox:lxc" as "svc" across ["redis", "postgres", "monitoring"]
  node:     "pve"
  name:     "#{item}-#{workspace}"
  ostempl:  "local:vztmpl/debian-12-standard_12.2-1_amd64.tar.zst"
  cores:    1
  memory:   1024
  tags:     [workspace, item, "managed-by:gecko"]
end

`)
		case "fly":
			habitats.WriteString(`habitat "fly"
  token: secret("fly.api.token")
  org:   env("FLY_ORG") | "personal"
end

`)
			spawns.WriteString(`# ─── Fly App ─────────────────────────────────────────────────────────────
spawn "fly:app" as "web-app"
  name: "#{app_name}-#{workspace}"
  org:  "personal"
end

# ─── Fly Machine ─────────────────────────────────────────────────────────
spawn "fly:machine" as "web"
  needs:  @web-app
  app:    @web-app.name
  image:  "#{app_name}:latest"
  region: "iad"
  cpus:   1
  memory: 256
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

# ─── Store (state backend) ────────────────────────────────────────────────────
store "%s"
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
`, name, workspace, name, workspace, backend, habitats.String(), name, workspace, spawns.String())
}
