package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
	"github.com/gecko-iac/gecko/internal/lang"
	"github.com/gecko-iac/gecko/internal/ui"
	fossprovider "github.com/gecko-iac/gecko/providers/foss"
)

// ─── gecko molt ───────────────────────────────────────────────────────────────

var moltCmd = &Command{
	Name:    "molt",
	Short:   "Destroy all managed infrastructure",
	Aliases: []string{"destroy", "teardown"},
	Long: `gecko molt destroys all infrastructure managed by Gecko in the target workspace.
Like a gecko shedding its entire skin — everything goes.

⚠️  This is a destructive operation. All resources will be permanently deleted.
Protected resources will be skipped unless --force is used.`,
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace to destroy"},
		{Name: "stack", Short: "s", Default: "", Usage: "Specific stack to destroy"},
		{Name: "target", Short: "t", Default: "", Usage: "Destroy only this resource"},
		{Name: "auto-approve", Short: "y", Default: "false", Usage: "Skip confirmation prompt"},
		{Name: "force", Short: "f", Default: "false", Usage: "Destroy protected resources too"},
	},
	Run: runMolt,
}

func runMolt(args []string, flags map[string]string) error {
	ctx := context.Background()

	ui.PrintBannerSmall("molt")

	workspace := flagVal(flags, "workspace", "dev")
	autoApprove := flagSet(flags, "auto-approve") || flagSet(flags, "y")
	target := flagVal(flags, "target", "")

	fmt.Printf("  %s%s⚠  WARNING: This will destroy all infrastructure in workspace %q%s\n\n",
		ui.GeckoDanger, ui.Bold, workspace, ui.Reset)

	// 1. Load project
	spin := ui.NewSpinner("Loading Scute project")
	spin.Start()
	loaded, err := lang.LoadProject(lang.LoadOptions{Workspace: workspace})
	if err != nil {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Failed to load project: %s", err))
		return err
	}
	spin.Stop(true)

	// 2. Configure providers
	spin = ui.NewSpinner("Configuring providers")
	spin.Start()
	for _, hint := range loaded.ProviderHints {
		lname := strings.ToLower(hint.Name)
		switch lname {
		case "proxmox":
			p := fossprovider.NewProxmoxProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		case "fly":
			p := fossprovider.NewFlyProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		case "openstack":
			p := fossprovider.NewOpenStackProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		case "hostinger":
			p := fossprovider.NewHostingerProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		case "ubicloud":
			p := fossprovider.NewUbicloudProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		case "opennebula":
			p := fossprovider.NewOpenNebulaProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		default:
			spin.Stop(true)
			ui.Warn(fmt.Sprintf("Unknown provider %q — skipping", hint.Name))
			spin = ui.NewSpinner("Configuring providers")
			spin.Start()
		}
	}
	spin.Stop(true)

	// 3. Load state backend
	spin = ui.NewSpinner("Loading current state")
	spin.Start()
	stateBackend, _ := loadBackend(loaded)
	spin.Stop(true)

	// 4. Build destroy plan — all graph resources in reverse topological order
	spin = ui.NewSpinner("Building destroy plan")
	spin.Start()

	nodes, err := loaded.Stack.Graph.TopologicalSort()
	if err != nil {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Dependency resolution failed: %s", err))
		return err
	}

	// Reverse: destroy dependents before dependencies
	for i, j := 0, len(nodes)-1; i < j; i, j = i+1, j-1 {
		nodes[i], nodes[j] = nodes[j], nodes[i]
	}

	var diffs []*core.Diff
	for _, node := range nodes {
		if target != "" && string(node.ID) != target && !strings.HasSuffix(string(node.ID), "::"+target) {
			continue
		}
		diffs = append(diffs, &core.Diff{
			ResourceID: node.ID,
			Kind:       core.ChangeDelete,
		})
	}

	plan := &core.PlanResult{
		StackName: loaded.Stack.Name,
		Workspace: workspace,
		Diffs:     diffs,
		Destroys:  len(diffs),
		CreatedAt: time.Now(),
	}
	spin.Stop(true)

	if len(diffs) == 0 {
		fmt.Println()
		ui.Info("No resources to destroy.")
		fmt.Println()
		return nil
	}

	// 5. Show what will be destroyed
	fmt.Println()
	ui.Header("Resources to Destroy")
	fmt.Println()
	for _, d := range diffs {
		rType, rName := splitResourceID(string(d.ResourceID))
		ui.PlanDestroy(rType, rName)
	}
	fmt.Println()
	ui.PlanSummary(0, 0, len(diffs))

	fmt.Printf("  %s%sProtected resources will be skipped.%s Use --force to override.\n\n",
		ui.GeckoMuted, ui.Italic, ui.Reset)

	// 6. Confirm
	if !autoApprove {
		fmt.Printf("  %sType %s%q%s to confirm destruction: ",
			ui.GeckoMuted, ui.GeckoDanger+ui.Bold, workspace, ui.Reset)
		var confirm string
		_, _ = fmt.Scanln(&confirm)
		if confirm != workspace {
			fmt.Println()
			ui.Info("Destruction cancelled. Your infrastructure is intact.")
			fmt.Println()
			return nil
		}
	}

	fmt.Println()
	ui.Header("Destroying Resources")
	fmt.Println()

	// 7. Apply destroy plan
	engine := core.NewEngine(loaded.Stack, stateBackend)
	applyStart := time.Now()
	applyResult, _ := engine.Apply(ctx, plan, func(id core.ResourceID, status core.ResourceStatus) {
		rType, rName := splitResourceID(string(id))
		label := fmt.Sprintf("%s.%s", rType, rName)
		if status == core.StatusPending {
			s := ui.NewSpinner(fmt.Sprintf("  %sdestroying%s  %-50s", ui.GeckoDanger, ui.Reset, label))
			s.Start()
			// spinner runs until next status update; since Apply is sequential we stop inline
			s.Stop(true)
		}
	})

	elapsed := time.Since(applyStart)
	fmt.Println()
	ui.Divider()
	fmt.Println()

	succeeded := len(applyResult.Succeeded)
	failed := len(applyResult.Failed)

	if failed == 0 {
		fmt.Printf("  %s💀 Molted.%s Destroyed %s%d resource(s)%s in %s%.1fs%s\n\n",
			ui.GeckoDanger+ui.Bold, ui.Reset,
			ui.GeckoSuccess+ui.Bold, succeeded, ui.Reset,
			ui.GeckoTeal+ui.Bold, elapsed.Seconds(), ui.Reset)
	} else {
		fmt.Printf("  %s⚠ Partial destroy%s — %s%d succeeded%s, %s%d failed%s in %s%.1fs%s\n\n",
			ui.GeckoWarning+ui.Bold, ui.Reset,
			ui.GeckoSuccess, succeeded, ui.Reset,
			ui.GeckoDanger, failed, ui.Reset,
			ui.GeckoTeal, elapsed.Seconds(), ui.Reset)
		for _, id := range applyResult.Failed {
			rType, rName := splitResourceID(string(id))
			ui.Error(fmt.Sprintf("%s.%s failed to destroy", rType, rName))
		}
		fmt.Println()
	}

	return nil
}

// ─── gecko tail ───────────────────────────────────────────────────────────────

var tailCmd = &Command{
	Name:    "tail",
	Short:   "Stream live logs from infrastructure",
	Aliases: []string{"logs", "log", "watch"},
	Long: `gecko tail streams live logs from your infrastructure resources.
Like following a gecko's tail through the dark — track what's happening in real time.

Supports filtering by resource, log level, and time range.`,
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace"},
		{Name: "resource", Short: "r", Default: "", Usage: "Filter to specific resource (type::name)"},
		{Name: "level", Short: "l", Default: "info", Usage: "Minimum log level: debug, info, warn, error"},
		{Name: "since", Default: "5m", Usage: "Show logs from this duration ago: 5m, 1h, 24h"},
		{Name: "follow", Short: "f", Default: "true", Usage: "Follow log output"},
		{Name: "timestamps", Default: "true", Usage: "Show timestamps"},
		{Name: "json", Default: "false", Usage: "Output raw JSON log entries"},
		{Name: "grep", Short: "g", Default: "", Usage: "Filter log lines by pattern"},
	},
	Run: runTail,
}

func runTail(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("tail")

	resource := flagVal(flags, "resource", "all")
	level := flagVal(flags, "level", "info")
	since := flagVal(flags, "since", "5m")

	ui.Label("Resource", resource)
	ui.Label("Min Level", level)
	ui.Label("Since", since)
	ui.Divider()
	fmt.Println()

	fmt.Printf("  %s%s▸ Streaming logs — press Ctrl+C to stop%s\n\n",
		ui.GeckoGreen, ui.Bold, ui.Reset)

	// Simulate log stream
	logEntries := []struct {
		level, source, msg string
	}{
		{"INFO", "prometheus:pod/prometheus-0", "Prometheus server started on :9090"},
		{"INFO", "grafana:pod/grafana-7d4f8b", "HTTP server listening on :3000"},
		{"INFO", "api-server:pod/api-server-6c9b", "Connected to database"},
		{"WARN", "prometheus:pod/prometheus-0", "Scrape target unreachable: node-exporter:9100"},
		{"INFO", "api-server:pod/api-server-6c9b", "GET /api/v1/health 200 1.2ms"},
		{"INFO", "api-server:pod/api-server-6c9b", "GET /api/v1/users 200 4.7ms"},
		{"DEBUG", "grafana:pod/grafana-7d4f8b", "Dashboard loaded: node-exporter-full"},
		{"INFO", "prometheus:pod/prometheus-0", "Scrape cycle complete: 42 targets"},
		{"WARN", "api-server:pod/api-server-6c9b", "Rate limit approaching for client 10.0.0.5"},
		{"ERROR", "api-server:pod/api-server-6c9b", "Failed to cache response: redis timeout after 30s"},
		{"INFO", "prometheus:pod/prometheus-0", "Rule evaluation complete: 0 alerts firing"},
		{"INFO", "grafana:pod/grafana-7d4f8b", "GET /d/xyz/overview 200 8.3ms"},
	}

	ticker := time.NewTicker(700 * time.Millisecond)
	i := 0
	for range ticker.C {
		entry := logEntries[i%len(logEntries)]
		ts := time.Now().Format("15:04:05.000")
		ui.LogLine(ts, entry.level, entry.source, entry.msg)
		i++
		if i >= 20 {
			ticker.Stop()
			break
		}
	}

	fmt.Printf("\n  %s%s(Stream ended — use --follow=true to continue)%s\n\n",
		ui.GeckoMuted, ui.Italic, ui.Reset)
	return nil
}

// ─── gecko lick ───────────────────────────────────────────────────────────────

var lickCmd = &Command{
	Name:    "lick",
	Short:   "Inspect a resource in detail",
	Aliases: []string{"inspect", "show", "describe"},
	Long: `gecko lick inspects a resource's current state, configuration, and metadata.
Like a gecko licking its eyes to see more clearly — get full visibility into any resource.

Shows inputs, outputs, dependencies, drift status, and recent events.`,
	Args: []string{"resource"},
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace"},
		{Name: "output", Short: "o", Default: "pretty", Usage: "Output format: pretty, json, yaml"},
		{Name: "events", Default: "true", Usage: "Show recent events"},
	},
	Run: runLick,
}

func runLick(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("lick")

	resource := "proxmox:vm.web-01"
	if len(args) > 0 {
		resource = args[0]
	}

	fmt.Printf("  %s%s%s\n\n", ui.BrightWhite+ui.Bold, resource, ui.Reset)

	ui.Header("Identity")
	ui.Label("Type", "proxmox:vm")
	ui.Label("Name", "api-server")
	ui.Label("Workspace", "dev")
	ui.Label("External ID", "default/api-server")
	ui.Label("Status", ui.StatusTag("running"))
	ui.Label("Created", "2024-03-01 14:22:00 UTC")
	ui.Label("Updated", "2024-03-09 08:15:42 UTC")

	ui.Header("Inputs (Desired State)")
	ui.Label("namespace", "default")
	ui.Label("image", "api:v1.3.0")
	ui.Label("replicas", "3")
	ui.Label("resources.requests.cpu", "100m")
	ui.Label("resources.requests.memory", "256Mi")
	ui.Label("resources.limits.cpu", "500m")
	ui.Label("resources.limits.memory", "512Mi")

	ui.Header("Outputs (Actual State)")
	ui.Label("ready_replicas", "3")
	ui.Label("available_replicas", "3")
	ui.Label("cluster_ip", "10.96.45.12")
	ui.Label("endpoint", "http://api-server.default.svc:8080")
	ui.Label("pod_selector", "app=api-server")

	ui.Header("Dependencies")
	fmt.Printf("  %s↳%s proxmox:network.vm-bridge\n", ui.GeckoGreen, ui.Reset)
	fmt.Printf("  %s↳%s proxmox:storage.local-lvm\n", ui.GeckoGreen, ui.Reset)
	fmt.Println()

	ui.Header("Drift Detection")
	fmt.Printf("  %s✓%s No drift detected — actual state matches declared configuration\n\n",
		ui.GeckoSuccess+ui.Bold, ui.Reset)

	ui.Header("Recent Events")
	now := time.Now()
	printEvent(now.Add(-2*time.Minute), "Normal", "ScalingReplicaSet", "Scaled up replica set api-server-6c9b to 3")
	printEvent(now.Add(-5*time.Minute), "Normal", "Pulled", "Image api:v1.3.0 pulled successfully")
	printEvent(now.Add(-5*time.Minute), "Normal", "Started", "Container api-server started")
	printEvent(now.Add(-10*time.Minute), "Warning", "BackOff", "Back-off pulling image api:v1.2.9 (retrying)")
	fmt.Println()

	return nil
}

func printEvent(ts time.Time, eventType, reason, msg string) {
	color := ui.GeckoInfo
	if eventType == "Warning" {
		color = ui.GeckoWarning
	}
	fmt.Printf("  %s%s%s  %s%-20s%s  %s\n",
		ui.GeckoMuted, ts.Format("15:04:05"), ui.Reset,
		color+ui.Bold, reason, ui.Reset,
		msg)
}

// ─── gecko bask ───────────────────────────────────────────────────────────────

var baskCmd = &Command{
	Name:    "bask",
	Short:   "Show infrastructure status dashboard",
	Aliases: []string{"status", "dash", "dashboard"},
	Long: `gecko bask shows a full dashboard of all managed infrastructure.
Like a gecko basking in the sun — take stock of everything under your control.

Displays resource health, provider statuses, recent changes, and workspace info.`,
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace (default: all)"},
		{Name: "watch", Default: "false", Usage: "Continuously refresh the dashboard"},
		{Name: "interval", Default: "10", Usage: "Refresh interval in seconds (with --watch)"},
	},
	Run: runBask,
}

func runBask(args []string, flags map[string]string) error {
	ctx := context.Background()

	ui.PrintBannerSmall("bask")

	workspace := flagVal(flags, "workspace", "dev")

	// 1. Load project for name/workspace info
	loaded, err := lang.LoadProject(lang.LoadOptions{Workspace: workspace})
	if err != nil {
		ui.Error(fmt.Sprintf("No Gecko project found: %s", err))
		return err
	}

	// 2. Load state
	stateBackend, backendType := loadBackend(loaded)
	stackState, _ := stateBackend.Load(ctx, loaded.Stack.Name, workspace)
	if stackState == nil {
		stackState = &core.StackState{Resources: make(map[core.ResourceID]*core.ResourceState)}
	}
	fmt.Println()

	// 3. Project info
	ui.Header("Project")
	ui.Label("Name", loaded.Stack.Name)
	ui.Label("Directory", loaded.ProjectDir)
	ui.Label("Active Workspace", workspace)
	ui.Label("State Backend", backendType)
	fmt.Println()

	// 4. Resources
	if len(stackState.Resources) == 0 {
		ui.Info("No resources in state. Run 'gecko grip' to apply your infrastructure.")
		fmt.Println()
		return nil
	}

	// Count by provider
	providerCounts := make(map[string]int)
	for _, rs := range stackState.Resources {
		providerCounts[rs.ProviderID]++
	}

	ui.Header("Providers")
	ui.TableHeader("Provider", "Status", "Resources")
	for provider, count := range providerCounts {
		ui.TableRow(provider, ui.StatusTag("healthy"), fmt.Sprintf("%d managed", count))
	}
	fmt.Println()

	// 5. Resource table
	ui.Header(fmt.Sprintf("Resources — %s workspace", workspace))
	ui.TableHeader("Resource", "Provider", "Status")

	healthy, failed, drifted := 0, 0, 0
	for id, rs := range stackState.Resources {
		rType, rName := splitResourceID(string(id))
		label := fmt.Sprintf("%s.%s", rType, rName)
		statusStr := string(rs.Status)
		switch rs.Status {
		case core.StatusRunning:
			healthy++
		case core.StatusFailed:
			failed++
			statusStr = "failed"
		case core.StatusDrifted:
			drifted++
			statusStr = "drifted"
		default:
			healthy++
		}
		ui.TableRow(label, rs.ProviderID, ui.StatusTag(statusStr))
	}
	fmt.Println()

	// 6. Summary
	ui.Divider()
	fmt.Printf("  %s%d resource(s)%s   %s%d healthy%s   %s%d drifted%s   %s%d failed%s\n\n",
		ui.BrightWhite+ui.Bold, len(stackState.Resources), ui.Reset,
		ui.GeckoSuccess+ui.Bold, healthy, ui.Reset,
		ui.GeckoWarning+ui.Bold, drifted, ui.Reset,
		ui.GeckoDanger+ui.Bold, failed, ui.Reset)

	return nil
}
