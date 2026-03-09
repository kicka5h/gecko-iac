package cmd

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/gecko-iac/gecko/internal/ui"
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
	ui.PrintBannerSmall("molt")

	workspace := flagVal(flags, "workspace", "dev")
	autoApprove := flagSet(flags, "auto-approve") || flagSet(flags, "y")

	fmt.Printf("  %s%s⚠  WARNING: This will destroy all infrastructure in workspace %q%s\n\n",
		ui.GeckoDanger, ui.Bold, workspace, ui.Reset)

	ui.Header("Resources to Destroy")
	fmt.Println()
	ui.PlanDestroy("k8s:service", "grafana-svc")
	ui.PlanDestroy("k8s:service", "prometheus-svc")
	ui.PlanDestroy("k8s:deployment", "grafana")
	ui.PlanDestroy("k8s:deployment", "prometheus")
	ui.PlanDestroy("k8s:persistentvolumeclaim", "grafana-pvc")
	ui.PlanDestroy("k8s:configmap", "prometheus-config")
	ui.PlanDestroy("k8s:configmap", "app-config")
	ui.PlanDestroy("k8s:deployment", "api-server")
	ui.PlanDestroy("k8s:namespace", "monitoring")

	ui.PlanSummary(0, 0, 9)

	fmt.Printf("  %s%sProtected resources will be skipped.%s Use --force to override.\n\n",
		ui.GeckoMuted, ui.Italic, ui.Reset)

	if !autoApprove {
		fmt.Printf("  %sType %s%q%s to confirm destruction: ", ui.GeckoMuted, ui.GeckoDanger+ui.Bold, workspace, ui.Reset)
		var confirm string
		fmt.Scanln(&confirm)
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

	destroySteps := []string{
		"k8s:service.grafana-svc",
		"k8s:service.prometheus-svc",
		"k8s:deployment.grafana",
		"k8s:deployment.prometheus",
		"k8s:deployment.api-server",
		"k8s:persistentvolumeclaim.grafana-pvc",
		"k8s:configmap.prometheus-config",
		"k8s:configmap.app-config",
		"k8s:namespace.monitoring",
	}

	for i, r := range destroySteps {
		s := ui.NewSpinner(fmt.Sprintf("  %s  destroying  %s%s%s", ui.GeckoDanger, ui.BrightWhite, r, ui.Reset))
		s.Start()
		time.Sleep(time.Duration(400+rand.Intn(600)) * time.Millisecond)
		s.Stop(true)
		ui.ProgressBar(i+1, len(destroySteps), fmt.Sprintf(" %d/%d destroyed", i+1, len(destroySteps)))
	}

	fmt.Println()
	ui.Divider()
	fmt.Printf("\n  %s💀 Molted.%s All managed resources in %s%q%s have been destroyed.\n\n",
		ui.GeckoDanger+ui.Bold, ui.Reset,
		ui.GeckoTeal, workspace, ui.Reset)
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

	resource := "k8s:deployment.api-server"
	if len(args) > 0 {
		resource = args[0]
	}

	fmt.Printf("  %s%s%s\n\n", ui.BrightWhite+ui.Bold, resource, ui.Reset)

	ui.Header("Identity")
	ui.Label("Type", "k8s:deployment")
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
	fmt.Printf("  %s↳%s k8s:namespace.default\n", ui.GeckoGreen, ui.Reset)
	fmt.Printf("  %s↳%s k8s:configmap.app-config\n", ui.GeckoGreen, ui.Reset)
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
	ui.PrintBannerSmall("bask")

	spin := ui.NewSpinner("Fetching resource states")
	spin.Start()
	time.Sleep(800 * time.Millisecond)
	spin.Stop(true)
	fmt.Println()

	// Project info
	ui.Header("Project")
	ui.Label("Name", "my-homelab")
	ui.Label("Active Workspace", "dev")
	ui.Label("Workspaces", "dev, staging, prod")
	ui.Label("State Backend", "local")
	ui.Label("Last Applied", "2 minutes ago")

	// Provider health
	ui.Header("Providers")
	ui.TableHeader("Provider", "Version", "Status", "Resources")
	ui.TableRow("kubernetes", "v1.29.2", ui.StatusTag("healthy"), "18 managed")
	ui.TableRow("proxmox", "v8.1.3", ui.StatusTag("healthy"), "4 managed")
	ui.TableRow("gitea", "v1.21.4", ui.StatusTag("healthy"), "3 managed")
	fmt.Println()

	// Resource summary by type
	ui.Header("Resources — dev workspace")
	ui.TableHeader("Resource", "Provider", "Status", "Drift")
	ui.TableRow("k8s:namespace.monitoring", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("k8s:deployment.prometheus", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("k8s:deployment.grafana", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("k8s:deployment.api-server", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("k8s:service.prometheus-svc", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("k8s:service.grafana-svc", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("k8s:pvc.grafana-pvc", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("k8s:configmap.prometheus-config", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("k8s:configmap.app-config", "kubernetes", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("proxmox:vm.control-plane-01", "proxmox", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("proxmox:vm.worker-01", "proxmox", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("proxmox:vm.worker-02", "proxmox", ui.StatusTag("running"), "✓ clean")
	ui.TableRow("gitea:repo.infra", "gitea", ui.StatusTag("active"), "✓ clean")
	fmt.Println()

	// Health summary
	ui.Divider()
	fmt.Printf("  %s13 resources%s   %s13 healthy%s   %s0 drifted%s   %s0 failed%s\n\n",
		ui.BrightWhite+ui.Bold, ui.Reset,
		ui.GeckoSuccess+ui.Bold, ui.Reset,
		ui.GeckoWarning+ui.Bold, ui.Reset,
		ui.GeckoDanger+ui.Bold, ui.Reset)

	return nil
}
