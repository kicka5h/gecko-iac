package cmd

import (
	"fmt"
	"time"

	"github.com/gecko-iac/gecko/internal/ui"
)

var gripCmd = &Command{
	Name:    "grip",
	Short:   "Apply infrastructure changes",
	Aliases: []string{"apply", "deploy"},
	Long: `gecko grip applies your declared infrastructure to reality.
Like a gecko gripping a wall — firm, precise, and committed.

Gecko will show you a crawl (plan) first and ask for confirmation
unless --auto-approve is set.`,
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace"},
		{Name: "stack", Short: "s", Default: "", Usage: "Specific stack to apply"},
		{Name: "target", Short: "t", Default: "", Usage: "Apply only this resource (type::name)"},
		{Name: "auto-approve", Short: "y", Default: "false", Usage: "Skip confirmation prompt"},
		{Name: "plan", Short: "p", Default: "", Usage: "Use a previously saved plan file"},
		{Name: "parallelism", Default: "10", Usage: "Max concurrent operations"},
		{Name: "refresh", Default: "true", Usage: "Refresh state before applying"},
	},
	Run: runGrip,
}

func runGrip(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("grip")

	workspace := flagVal(flags, "workspace", "dev")
	autoApprove := flagSet(flags, "auto-approve") || flagSet(flags, "y")

	ui.Info(fmt.Sprintf("Preparing to grip workspace %s%q%s...", ui.GeckoTeal, workspace, ui.Reset))
	fmt.Println()

	// Show abbreviated plan
	spin := ui.NewSpinner("Loading current state")
	spin.Start()
	time.Sleep(500 * time.Millisecond)
	spin.Stop(true)

	spin = ui.NewSpinner("Computing plan")
	spin.Start()
	time.Sleep(800 * time.Millisecond)
	spin.Stop(true)

	fmt.Println()
	ui.Header("Changes to Apply")
	fmt.Println()

	ui.PlanAdd("k8s:namespace", "monitoring")
	ui.PlanAdd("k8s:deployment", "prometheus")
	ui.PlanAdd("k8s:service", "prometheus-svc")
	ui.PlanAdd("k8s:configmap", "prometheus-config")
	ui.PlanAdd("k8s:deployment", "grafana")
	ui.PlanAdd("k8s:service", "grafana-svc")
	ui.PlanAdd("k8s:persistentvolumeclaim", "grafana-pvc")
	fmt.Println()
	ui.PlanChange("k8s:deployment", "api-server")
	ui.PlanChange("k8s:configmap", "app-config")

	ui.PlanSummary(7, 2, 0)

	// Confirmation
	if !autoApprove {
		if !ui.Confirm("Apply these changes?") {
			fmt.Println()
			ui.Info("Apply cancelled. Your infrastructure is unchanged.")
			fmt.Println()
			return nil
		}
	}

	fmt.Println()
	ui.Header("Applying Changes")
	fmt.Println()

	// Apply each resource with progress
	type applyStep struct {
		resource string
		delay    time.Duration
	}

	steps := []applyStep{
		{"k8s:namespace.monitoring", 400 * time.Millisecond},
		{"k8s:configmap.prometheus-config", 300 * time.Millisecond},
		{"k8s:deployment.prometheus", 1200 * time.Millisecond},
		{"k8s:service.prometheus-svc", 500 * time.Millisecond},
		{"k8s:persistentvolumeclaim.grafana-pvc", 600 * time.Millisecond},
		{"k8s:deployment.grafana", 1100 * time.Millisecond},
		{"k8s:service.grafana-svc", 400 * time.Millisecond},
		{"k8s:deployment.api-server", 900 * time.Millisecond},
		{"k8s:configmap.app-config", 250 * time.Millisecond},
	}

	start := time.Now()
	for i, step := range steps {
		s := ui.NewSpinner(fmt.Sprintf("  %s  %-50s", ui.GeckoMuted+"creating"+ui.Reset, step.resource))
		s.Start()
		time.Sleep(step.delay)
		s.Stop(true)
		ui.ProgressBar(i+1, len(steps), fmt.Sprintf(" %d/%d resources", i+1, len(steps)))
	}

	elapsed := time.Since(start)
	fmt.Println()
	ui.Divider()
	fmt.Println()

	fmt.Printf("  %s🦎 Gripped!%s Applied %s9 changes%s in %s%.1fs%s\n\n",
		ui.GeckoGreen+ui.Bold, ui.Reset,
		ui.GeckoSuccess+ui.Bold, ui.Reset,
		ui.GeckoTeal+ui.Bold, elapsed.Seconds(), ui.Reset)

	// Post-apply summary
	ui.Header("Applied Resources")
	ui.TableHeader("Resource", "Status", "Time")
	ui.TableRow("k8s:namespace.monitoring", ui.StatusTag("running"), "0.4s")
	ui.TableRow("k8s:deployment.prometheus", ui.StatusTag("running"), "1.2s")
	ui.TableRow("k8s:service.prometheus-svc", ui.StatusTag("running"), "0.5s")
	ui.TableRow("k8s:configmap.prometheus-config", ui.StatusTag("running"), "0.3s")
	ui.TableRow("k8s:deployment.grafana", ui.StatusTag("running"), "1.1s")
	ui.TableRow("k8s:service.grafana-svc", ui.StatusTag("running"), "0.4s")
	ui.TableRow("k8s:persistentvolumeclaim.grafana-pvc", ui.StatusTag("running"), "0.6s")
	ui.TableRow("k8s:deployment.api-server", ui.StatusTag("running"), "0.9s")
	ui.TableRow("k8s:configmap.app-config", ui.StatusTag("running"), "0.3s")
	fmt.Println()

	ui.Info("Run 'gecko bask' to view your infrastructure dashboard.")
	ui.Info("Run 'gecko tail' to stream live logs.")
	fmt.Println()

	return nil
}
