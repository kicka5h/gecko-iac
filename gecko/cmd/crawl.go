package cmd

import (
	"fmt"
	"time"

	"github.com/gecko-iac/gecko/internal/ui"
)

var crawlCmd = &Command{
	Name:    "crawl",
	Short:   "Preview infrastructure changes (plan)",
	Aliases: []string{"plan", "preview"},
	Long: `gecko crawl computes the difference between your declared infrastructure
and the current real-world state, showing exactly what would change.

Like a gecko crawling cautiously before striking — survey the terrain before
committing. No changes are made during a crawl.`,
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "", Usage: "Target workspace (overrides gecko.json)"},
		{Name: "stack", Short: "s", Default: "", Usage: "Specific stack to plan"},
		{Name: "target", Short: "t", Default: "", Usage: "Plan only this resource (type::name)"},
		{Name: "save", Default: "false", Usage: "Save plan to .gecko/plans/ for later use with gecko grip"},
		{Name: "json", Default: "false", Usage: "Output plan as JSON"},
		{Name: "diff", Default: "true", Usage: "Show field-level diffs"},
		{Name: "compact", Default: "false", Usage: "Compact output (no field diffs)"},
	},
	Run: runCrawl,
}

func runCrawl(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("crawl")

	workspace := flagVal(flags, "workspace", "dev")
	target := flagVal(flags, "target", "")
	showDiff := !flagSet(flags, "compact")

	ui.Info(fmt.Sprintf("Planning workspace %s%q%s...", ui.GeckoTeal, workspace, ui.Reset))
	if target != "" {
		ui.Info(fmt.Sprintf("Target: %s", target))
	}
	fmt.Println()

	// Simulate loading state
	spin := ui.NewSpinner("Loading current state")
	spin.Start()
	time.Sleep(600 * time.Millisecond)
	spin.Stop(true)

	spin = ui.NewSpinner("Refreshing resource states from providers")
	spin.Start()
	time.Sleep(900 * time.Millisecond)
	spin.Stop(true)

	spin = ui.NewSpinner("Computing resource dependency graph")
	spin.Start()
	time.Sleep(400 * time.Millisecond)
	spin.Stop(true)

	spin = ui.NewSpinner("Calculating diffs")
	spin.Start()
	time.Sleep(700 * time.Millisecond)
	spin.Stop(true)

	fmt.Println()

	// Mock plan output — in a real run this comes from engine.Plan()
	ui.Header("Resource Changes")
	fmt.Println()

	// Additions
	ui.PlanAdd("k8s:namespace", "monitoring")
	ui.PlanAdd("k8s:deployment", "prometheus")
	ui.PlanAdd("k8s:service", "prometheus-svc")
	ui.PlanAdd("k8s:configmap", "prometheus-config")
	ui.PlanAdd("k8s:deployment", "grafana")
	ui.PlanAdd("k8s:service", "grafana-svc")
	ui.PlanAdd("k8s:persistentvolumeclaim", "grafana-pvc")

	// Changes
	fmt.Println()
	ui.PlanChange("k8s:deployment", "api-server")
	ui.PlanChange("k8s:configmap", "app-config")

	// No-ops
	fmt.Println()
	ui.PlanNoChange("k8s:namespace", "default")
	ui.PlanNoChange("k8s:namespace", "kube-system")

	if showDiff {
		fmt.Println()
		ui.Header("Field-Level Diffs")
		fmt.Println()
		printFieldDiff("k8s:deployment.api-server", []fieldDiff{
			{"spec.replicas", "2", "3", false},
			{"spec.template.spec.containers[0].image", "api:v1.2.0", "api:v1.3.0", false},
			{"spec.template.spec.containers[0].resources.limits.memory", "256Mi", "512Mi", false},
		})

		printFieldDiff("k8s:configmap.app-config", []fieldDiff{
			{"data.LOG_LEVEL", "info", "debug", false},
			{"data.CACHE_TTL", "(not set)", "300", false},
		})
	}

	// Summary
	ui.PlanSummary(7, 2, 0)

	fmt.Printf("  %s%sNote:%s Gecko will make the changes above when you run %sgecko grip%s\n\n",
		ui.GeckoMuted, ui.Italic, ui.Reset,
		ui.GeckoTeal, ui.Reset)

	if flagSet(flags, "save") {
		planFile := fmt.Sprintf(".gecko/plans/plan-%d.json", time.Now().Unix())
		ui.Success(fmt.Sprintf("Plan saved to %s", planFile))
		fmt.Println()
	}

	return nil
}

type fieldDiff struct {
	field    string
	oldVal   string
	newVal   string
	computed bool
}

func printFieldDiff(resource string, diffs []fieldDiff) {
	fmt.Printf("  %s%s%s\n", ui.BrightWhite+ui.Bold, resource, ui.Reset)
	for _, d := range diffs {
		if d.computed {
			fmt.Printf("     %s%-45s%s %s(known after apply)%s\n",
				ui.GeckoMuted, d.field, ui.Reset,
				ui.GeckoWarning+ui.Italic, ui.Reset)
		} else {
			fmt.Printf("     %s%-45s%s %s%s%s → %s%s%s\n",
				ui.GeckoMuted, d.field, ui.Reset,
				ui.GeckoDanger, d.oldVal, ui.Reset,
				ui.GeckoSuccess, d.newVal, ui.Reset)
		}
	}
	fmt.Println()
}
