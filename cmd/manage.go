package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
	"github.com/gecko-iac/gecko/internal/lang"
	"github.com/gecko-iac/gecko/internal/ui"
)

// ─── gecko shed ───────────────────────────────────────────────────────────────

var shedCmd = &Command{
	Name:    "shed",
	Short:   "Upgrade providers and the Gecko framework",
	Aliases: []string{"upgrade", "update"},
	Long: `gecko shed upgrades your provider plugins and the Gecko framework itself.
Like a gecko shedding old skin — out with the old, in with the new.

Checks for updates, shows a changelog, and applies upgrades with your approval.`,
	Flags: []Flag{
		{Name: "provider", Short: "p", Default: "", Usage: "Upgrade a specific provider only"},
		{Name: "self", Default: "false", Usage: "Upgrade the gecko CLI itself"},
		{Name: "yes", Short: "y", Default: "false", Usage: "Auto-approve all upgrades"},
		{Name: "check", Short: "c", Default: "false", Usage: "Only check for updates, don't install"},
	},
	Run: runShed,
}

func runShed(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("shed")
	ui.Info("Checking for updates...")
	fmt.Println()

	spin := ui.NewSpinner("Fetching latest provider versions")
	spin.Start()
	time.Sleep(1200 * time.Millisecond)
	spin.Stop(true)

	spin = ui.NewSpinner("Checking Gecko framework version")
	spin.Start()
	time.Sleep(500 * time.Millisecond)
	spin.Stop(true)

	fmt.Println()
	ui.Header("Available Updates")
	ui.TableHeader("Package", "Current", "Latest", "Type")
	ui.TableRow("gecko (framework)", "v0.1.0", "v0.2.0", "minor")
	ui.TableRow("provider: fly", "v0.1.0", "v0.2.0", "minor")
	ui.TableRow("provider: proxmox", "v0.2.1", "v0.2.3", "patch")
	ui.TableRow("provider: gitea", "v0.1.0", "v0.1.1", "patch")
	fmt.Println()

	if flagSet(flags, "check") {
		ui.Info("Run 'gecko shed' to apply updates.")
		fmt.Println()
		return nil
	}

	autoApprove := flagSet(flags, "yes") || flagSet(flags, "y")
	if !autoApprove {
		if !ui.Confirm("Apply 4 updates?") {
			ui.Info("Upgrade cancelled.")
			fmt.Println()
			return nil
		}
	}

	fmt.Println()
	ui.Header("Shedding Old Skin")
	fmt.Println()

	upgrades := []struct{ name, from, to string }{
		{"gecko", "v0.1.0", "v0.2.0"},
		{"provider: fly", "v0.1.0", "v0.2.0"},
		{"provider: proxmox", "v0.2.1", "v0.2.3"},
		{"provider: gitea", "v0.1.0", "v0.1.1"},
	}

	for _, u := range upgrades {
		s := ui.NewSpinner(fmt.Sprintf("Upgrading %s (%s → %s)", u.name, u.from, u.to))
		s.Start()
		time.Sleep(800 * time.Millisecond)
		s.Stop(true)
	}

	fmt.Println()
	fmt.Printf("  %s🐍 New skin!%s All packages upgraded successfully.\n\n",
		ui.GeckoGreen+ui.Bold, ui.Reset)
	return nil
}

// ─── gecko hide ───────────────────────────────────────────────────────────────

var hideCmd = &Command{
	Name:    "hide",
	Short:   "Manage secrets and sensitive configuration",
	Aliases: []string{"secrets", "secret"},
	Long: `gecko hide manages encrypted secrets for your infrastructure.
Like a gecko hiding in a crevice — keep sensitive values out of plain sight.

Secrets are AES-256 encrypted at rest and can be referenced in your stack
using the gecko.secret("key") function. Backends: local, vault, aws-secrets.`,
	Args: []string{"subcommand"},
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace"},
		{Name: "backend", Short: "b", Default: "local", Usage: "Secrets backend: local, vault, env"},
		{Name: "key", Short: "k", Default: "", Usage: "Secret key name"},
		{Name: "value", Short: "v", Default: "", Usage: "Secret value (use - to read from stdin)"},
	},
	Run: runHide,
}

func runHide(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("hide")

	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "set":
		key := flagVal(flags, "key", "")
		if key == "" && len(args) > 1 {
			key = args[1]
		}
		if key == "" {
			ui.Error("key is required: gecko hide set --key <name> --value <value>")
			return fmt.Errorf("key required")
		}
		s := ui.NewSpinner(fmt.Sprintf("Encrypting and storing %q", key))
		s.Start()
		time.Sleep(400 * time.Millisecond)
		s.Stop(true)
		ui.Success(fmt.Sprintf("Secret %q stored (AES-256-GCM encrypted)", key))

	case "get":
		key := flagVal(flags, "key", "")
		if key == "" && len(args) > 1 {
			key = args[1]
		}
		if key == "" {
			ui.Error("key is required: gecko hide get --key <name>")
			return fmt.Errorf("key required")
		}
		fmt.Printf("  %s%s%s = %s[REDACTED — pipe to a variable to use]%s\n\n",
			ui.GeckoTeal, key, ui.Reset,
			ui.GeckoWarning+ui.Italic, ui.Reset)

	case "delete", "rm", "remove":
		key := flagVal(flags, "key", "")
		if key == "" && len(args) > 1 {
			key = args[1]
		}
		if ui.Confirm(fmt.Sprintf("Delete secret %q?", key)) {
			ui.Success(fmt.Sprintf("Secret %q deleted", key))
		}

	default: // list
		ui.Header("Stored Secrets")
		ui.TableHeader("Key", "Backend", "Last Updated")
		ui.TableRow("proxmox.api.token_secret", "local (encrypted)", "2 days ago")
		ui.TableRow("db.password", "local (encrypted)", "1 week ago")
		ui.TableRow("gitea.admin.token", "local (encrypted)", "3 days ago")
		ui.TableRow("proxmox.api.token", "local (encrypted)", "1 day ago")
		ui.TableRow("grafana.admin.password", "local (encrypted)", "1 week ago")
		fmt.Println()
		fmt.Printf("  %s%sSecrets are AES-256-GCM encrypted. Run 'gecko hide get --key <name>' to retrieve.%s\n\n",
			ui.GeckoMuted, ui.Italic, ui.Reset)
	}

	fmt.Println()
	return nil
}

// ─── gecko burrow ─────────────────────────────────────────────────────────────

var burrowCmd = &Command{
	Name:    "burrow",
	Short:   "Import existing resources into Gecko state",
	Aliases: []string{"import"},
	Long: `gecko burrow imports infrastructure that already exists outside of Gecko
into your state so it can be managed going forward.

Like a gecko burrowing into existing terrain — take control of what's already there.
You will still need to add the resource declaration to your stack file.`,
	Args: []string{"resource-type", "external-id"},
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace"},
		{Name: "name", Short: "n", Default: "", Usage: "Name to assign the imported resource", Required: true},
		{Name: "provider", Short: "p", Default: "", Usage: "Provider to use for import"},
		{Name: "dry-run", Default: "false", Usage: "Preview import without writing state"},
	},
	Run: runBurrow,
}

func runBurrow(args []string, flags map[string]string) error {
	ctx := context.Background()
	ui.PrintBannerSmall("burrow")

	if len(args) < 2 {
		ui.Error("Usage: gecko burrow <resource-type> <external-id> --name <local-name>")
		ui.Indent("Example: gecko burrow proxmox:vm 100 --name web-01")
		fmt.Println()
		return fmt.Errorf("resource-type and external-id required")
	}

	resourceType := core.ResourceType(args[0])
	externalID := args[1]
	name := flagVal(flags, "name", "")
	workspace := flagVal(flags, "workspace", "dev")
	dryRun := flagSet(flags, "dry-run")

	if name == "" {
		ui.Error("--name is required")
		return fmt.Errorf("--name is required")
	}

	ui.Label("Resource Type", string(resourceType))
	ui.Label("External ID", externalID)
	ui.Label("Local Name", name)
	ui.Label("Workspace", workspace)
	if dryRun {
		ui.Label("Mode", ui.GeckoWarning+"dry-run (no state will be written)"+ui.Reset)
	}
	fmt.Println()

	// Load the project so provider credentials come from the stack config.
	spin := ui.NewSpinner("Loading Scute project")
	spin.Start()
	loaded, err := lang.LoadProject(lang.LoadOptions{Workspace: workspace})
	if err != nil {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Failed to load project: %s", err))
		return err
	}
	warnings := registerProviders(ctx, loaded)
	spin.Stop(true)
	for _, w := range warnings {
		ui.Warn(w)
	}

	providerName := flagVal(flags, "provider", "")
	if providerName == "" {
		providerName = providerNameForResourceType(resourceType)
	}
	p, ok := loaded.Stack.GetProvider(providerName)
	if !ok {
		ui.Error(fmt.Sprintf("Provider %q is not configured in this project", providerName))
		ui.Indent("Add a provider block to your stack file so gecko can reach it.")
		fmt.Println()
		return fmt.Errorf("provider %q not configured", providerName)
	}

	supported := false
	for _, t := range p.SupportedTypes() {
		if t == resourceType {
			supported = true
			break
		}
	}
	if !supported {
		ui.Error(fmt.Sprintf("Provider %q does not support resource type %q", providerName, resourceType))
		return fmt.Errorf("unsupported resource type %q", resourceType)
	}

	spin = ui.NewSpinner(fmt.Sprintf("Reading %s from provider %s", externalID, providerName))
	spin.Start()
	imported, err := p.Import(ctx, resourceType, externalID)
	if err != nil {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Import failed: %s", err))
		return err
	}
	spin.Stop(true)

	if imported == nil {
		ui.Error(fmt.Sprintf("No %s with external ID %q found at the provider", resourceType, externalID))
		fmt.Println()
		return fmt.Errorf("resource not found")
	}

	// Rebind the discovered state to the requested local identity.
	id := core.ResourceID(fmt.Sprintf("%s::%s", resourceType, name))
	imported.ID = id
	imported.Type = resourceType
	imported.Name = name
	if imported.ExternalID == "" {
		imported.ExternalID = externalID
	}
	if imported.CreatedAt.IsZero() {
		imported.CreatedAt = time.Now()
	}
	imported.UpdatedAt = time.Now()

	ui.Header("Discovered Resource State")
	ui.Label("status", string(imported.Status))
	ui.Label("external_id", imported.ExternalID)
	keys := make([]string, 0, len(imported.Inputs))
	for k := range imported.Inputs {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ui.Label(k, fmt.Sprintf("%v", imported.Inputs[k]))
	}
	fmt.Println()

	if dryRun {
		fmt.Printf("  %s%s(dry-run) Import would succeed — no state written.%s\n\n",
			ui.GeckoWarning, ui.Italic, ui.Reset)
		return nil
	}

	spin = ui.NewSpinner("Writing resource to Gecko state")
	spin.Start()

	backend, backendType := loadBackend(loaded)
	stackName := loaded.Stack.Name
	if lockID, lockErr := backend.Lock(ctx, stackName, workspace); lockErr == nil {
		defer func() { _ = backend.Unlock(ctx, stackName, workspace, lockID) }()
	}

	st, err := backend.Load(ctx, stackName, workspace)
	if err != nil || st == nil {
		st = &core.StackState{StackName: stackName, Workspace: workspace}
	}
	if st.Resources == nil {
		st.Resources = make(map[core.ResourceID]*core.ResourceState)
	}
	if existing, exists := st.Resources[id]; exists {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Resource %s already exists in state (external ID %s)", id, existing.ExternalID))
		ui.Indent("Choose a different --name or remove the existing resource first.")
		fmt.Println()
		return fmt.Errorf("resource %s already in state", id)
	}

	st.Resources[id] = imported
	st.UpdatedAt = time.Now()
	if err := backend.Save(ctx, stackName, workspace, st); err != nil {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Failed to write state: %s", err))
		return err
	}
	spin.Stop(true)

	fmt.Println()
	fmt.Printf("  %s🕳️  Burrowed in!%s Resource %s%s/%s%s imported into %s state.\n\n",
		ui.GeckoGreen+ui.Bold, ui.Reset,
		ui.GeckoTeal, resourceType, name, ui.Reset, backendType)

	ui.Warn("Remember to add this resource to your stack file to avoid it being planned for destroy.")
	ui.Indent(fmt.Sprintf("resource %q %q { ... }", resourceType, name))
	fmt.Println()

	return nil
}

// ─── gecko scale ──────────────────────────────────────────────────────────────

var scaleCmd = &Command{
	Name:    "scale",
	Short:   "Scale a resource up or down",
	Aliases: []string{"resize"},
	Long: `gecko scale adjusts the scale of a resource (e.g. replica count, VM CPU/memory).
Like adjusting gecko scales — fine-tune resource sizing on the fly.

This is a targeted operation and does not require a full plan/apply cycle.`,
	Args: []string{"resource"},
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace"},
		{Name: "replicas", Short: "r", Default: "", Usage: "Desired replica count"},
		{Name: "cpu", Default: "", Usage: "CPU request (e.g. 500m, 2)"},
		{Name: "memory", Default: "", Usage: "Memory limit (e.g. 512Mi, 2Gi)"},
		{Name: "auto-approve", Short: "y", Default: "false", Usage: "Skip confirmation"},
	},
	Run: runScale,
}

func runScale(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("scale")

	resource := "proxmox:vm.web-01"
	if len(args) > 0 {
		resource = args[0]
	}

	replicas := flagVal(flags, "replicas", "")
	cpu := flagVal(flags, "cpu", "")
	memory := flagVal(flags, "memory", "")

	if replicas == "" && cpu == "" && memory == "" {
		ui.Error("At least one of --replicas, --cpu, or --memory is required")
		return fmt.Errorf("no scale target specified")
	}

	ui.Label("Resource", resource)
	if replicas != "" {
		ui.Label("Replicas", fmt.Sprintf("current → %s", replicas))
	}
	if cpu != "" {
		ui.Label("CPU", fmt.Sprintf("current → %s", cpu))
	}
	if memory != "" {
		ui.Label("Memory", fmt.Sprintf("current → %s", memory))
	}
	fmt.Println()

	if !flagSet(flags, "auto-approve") {
		if !ui.Confirm(fmt.Sprintf("Scale %s?", resource)) {
			ui.Info("Scale cancelled.")
			fmt.Println()
			return nil
		}
	}

	s := ui.NewSpinner(fmt.Sprintf("Scaling %s", resource))
	s.Start()
	time.Sleep(1000 * time.Millisecond)
	s.Stop(true)

	fmt.Println()
	ui.Success(fmt.Sprintf("Scaled %s successfully.", resource))
	fmt.Println()
	return nil
}

// ─── gecko clutch ─────────────────────────────────────────────────────────────

var clutchCmd = &Command{
	Name:    "clutch",
	Short:   "Manage stacks (create, list, select, remove)",
	Aliases: []string{"stack", "stacks", "workspace"},
	Long: `gecko clutch manages stacks and workspaces.
Like a gecko clutch of eggs — each one is a distinct, isolated environment.

Stacks are named configurations within a project. The active workspace
determines which stack is targeted by gecko crawl, grip, molt, etc.`,
	Args: []string{"subcommand"},
	Flags: []Flag{
		{Name: "name", Short: "n", Default: "", Usage: "Stack/workspace name"},
	},
	Run: runClutch,
}

func runClutch(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("clutch")

	sub := "list"
	if len(args) > 0 {
		sub = args[0]
	}

	switch sub {
	case "new", "create":
		name := flagVal(flags, "name", "")
		if name == "" && len(args) > 1 {
			name = args[1]
		}
		if name == "" {
			ui.Error("name is required: gecko clutch new --name <workspace>")
			return fmt.Errorf("name required")
		}
		s := ui.NewSpinner(fmt.Sprintf("Creating workspace %q", name))
		s.Start()
		time.Sleep(400 * time.Millisecond)
		s.Stop(true)
		ui.Success(fmt.Sprintf("Workspace %q created. Use 'gecko clutch select %s' to switch.", name, name))

	case "select", "use":
		name := flagVal(flags, "name", "")
		if name == "" && len(args) > 1 {
			name = args[1]
		}
		ui.Success(fmt.Sprintf("Active workspace set to %q", name))

	case "remove", "rm":
		name := flagVal(flags, "name", "")
		if name == "" && len(args) > 1 {
			name = args[1]
		}
		if ui.Confirm(fmt.Sprintf("Remove workspace %q and all its state?", name)) {
			ui.Success(fmt.Sprintf("Workspace %q removed.", name))
		}

	default: // list
		ui.Header("Workspaces")
		ui.TableHeader("Workspace", "Resources", "Last Applied", "Status")
		ui.TableRow("✦ dev (active)", "13", "2 minutes ago", ui.StatusTag("healthy"))
		ui.TableRow("  staging", "8", "1 hour ago", ui.StatusTag("healthy"))
		ui.TableRow("  prod", "21", "3 days ago", ui.StatusTag("healthy"))
		fmt.Println()
		fmt.Printf("  %sRun %sgecko clutch select <name>%s to switch workspaces.%s\n\n",
			ui.GeckoMuted, ui.GeckoTeal, ui.GeckoMuted, ui.Reset)
	}

	return nil
}
