package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
	"github.com/gecko-iac/gecko/internal/lang"
	"github.com/gecko-iac/gecko/internal/state"
	"github.com/gecko-iac/gecko/internal/ui"
	k8sprovider "github.com/gecko-iac/gecko/providers/foss"
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
	ctx := context.Background()

	ui.PrintBannerSmall("crawl")

	workspace := flagVal(flags, "workspace", "dev")
	target := flagVal(flags, "target", "")
	showDiff := !flagSet(flags, "compact")
	outputJSON := flagSet(flags, "json")

	if !outputJSON {
		ui.Info(fmt.Sprintf("Planning workspace %s%q%s...", ui.GeckoTeal, workspace, ui.Reset))
		if target != "" {
			ui.Info(fmt.Sprintf("Target: %s", target))
		}
		fmt.Println()
	}

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

	// 2. Instantiate and configure providers
	spin = ui.NewSpinner("Configuring providers")
	spin.Start()
	for _, hint := range loaded.ProviderHints {
		lname := strings.ToLower(hint.Name)
		switch lname {
		case "k8s", "kubernetes":
			p := k8sprovider.NewProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		case "kind":
			p := k8sprovider.NewKindProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		case "vault":
			p := k8sprovider.NewVaultProvider(hint.Config)
			if err := p.Configure(ctx, hint.Config); err != nil {
				spin.Stop(false)
				ui.Warn(fmt.Sprintf("Provider %q configure warning: %s (continuing)", hint.Name, err))
				spin = ui.NewSpinner("Configuring providers")
				spin.Start()
			}
			loaded.Stack.RegisterProvider(p)
		case "nomad":
			p := k8sprovider.NewNomadProvider(hint.Config)
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

	// 4. Create engine and run plan
	spin = ui.NewSpinner("Computing resource dependency graph")
	spin.Start()
	engine := core.NewEngine(loaded.Stack, stateBackend)
	spin.Stop(true)

	spin = ui.NewSpinner("Calculating diffs")
	spin.Start()
	plan, err := engine.Plan(ctx)
	if err != nil {
		spin.Stop(false)
		ui.Error(fmt.Sprintf("Plan failed: %s", err))
		return err
	}
	spin.Stop(true)

	// 5. Output results

	// JSON output mode
	if outputJSON {
		data, err := json.MarshalIndent(plan, "", "  ")
		if err != nil {
			return fmt.Errorf("failed to marshal plan to JSON: %w", err)
		}
		fmt.Println(string(data))
		return nil
	}

	fmt.Println()
	ui.Header("Resource Changes")
	fmt.Println()

	// Render diffs grouped by kind
	for _, diff := range plan.Diffs {
		// Extract type and name from ResourceID ("k8s:deployment::nginx")
		rType, rName := splitResourceID(string(diff.ResourceID))

		switch diff.Kind {
		case core.ChangeAdd:
			ui.PlanAdd(rType, rName)
		case core.ChangeUpdate, core.ChangeReplace:
			ui.PlanChange(rType, rName)
		case core.ChangeDelete:
			ui.PlanDestroy(rType, rName)
		case core.ChangeNoOp:
			ui.PlanNoChange(rType, rName)
		}
	}

	// Field-level diffs
	if showDiff {
		hasDiffs := false
		for _, diff := range plan.Diffs {
			if len(diff.Changes) > 0 {
				hasDiffs = true
				break
			}
		}

		if hasDiffs {
			fmt.Println()
			ui.Header("Field-Level Diffs")
			fmt.Println()

			for _, diff := range plan.Diffs {
				if len(diff.Changes) == 0 {
					continue
				}
				rType, rName := splitResourceID(string(diff.ResourceID))
				resource := rType + "." + rName
				fmt.Printf("  %s%s%s\n", ui.BrightWhite+ui.Bold, resource, ui.Reset)

				for _, fc := range diff.Changes {
					oldStr := fmt.Sprintf("%v", fc.OldValue)
					newStr := fmt.Sprintf("%v", fc.NewValue)
					if fc.Computed {
						fmt.Printf("     %s%-45s%s %s(known after apply)%s\n",
							ui.GeckoMuted, fc.Field, ui.Reset,
							ui.GeckoWarning+ui.Italic, ui.Reset)
					} else {
						fmt.Printf("     %s%-45s%s %s%s%s → %s%s%s\n",
							ui.GeckoMuted, fc.Field, ui.Reset,
							ui.GeckoDanger, oldStr, ui.Reset,
							ui.GeckoSuccess, newStr, ui.Reset)
					}
				}
				fmt.Println()
			}
		}
	}

	// Summary
	ui.PlanSummary(plan.Adds, plan.Updates, plan.Destroys)

	fmt.Printf("  %s%sNote:%s Gecko will make the changes above when you run %sgecko grip%s\n\n",
		ui.GeckoMuted, ui.Italic, ui.Reset,
		ui.GeckoTeal, ui.Reset)

	// 6. Save plan if requested
	if flagSet(flags, "save") {
		planDir := ".gecko/plans"
		if err := os.MkdirAll(planDir, 0750); err != nil {
			ui.Warn(fmt.Sprintf("Could not create plan directory: %s", err))
		} else {
			planFile := filepath.Join(planDir, fmt.Sprintf("plan-%d.json", time.Now().Unix()))
			data, err := json.MarshalIndent(plan, "", "  ")
			if err == nil {
				if err := os.WriteFile(planFile, data, 0600); err != nil {
					ui.Warn(fmt.Sprintf("Could not save plan: %s", err))
				} else {
					ui.Success(fmt.Sprintf("Plan saved to %s", planFile))
					fmt.Println()
				}
			}
		}
	}

	return nil
}

// splitResourceID splits "k8s:deployment::nginx" into ("k8s:deployment", "nginx")
func splitResourceID(id string) (resourceType, name string) {
	idx := strings.LastIndex(id, "::")
	if idx < 0 {
		return id, id
	}
	return id[:idx], id[idx+2:]
}

// emptyStateBackend is a no-op backend used when no real backend is available
type emptyStateBackend struct{}

func (e *emptyStateBackend) Load(_ context.Context, stackName, workspace string) (*core.StackState, error) {
	return &core.StackState{
		StackName: stackName,
		Workspace: workspace,
		Resources: make(map[core.ResourceID]*core.ResourceState),
	}, nil
}

func (e *emptyStateBackend) Save(_ context.Context, _, _ string, _ *core.StackState) error {
	return nil
}

func (e *emptyStateBackend) Delete(_ context.Context, _, _ string) error {
	return nil
}

func (e *emptyStateBackend) List(_ context.Context) ([]string, error) {
	return nil, nil
}

func (e *emptyStateBackend) Lock(_ context.Context, _, _ string) (string, error) {
	return "noop-lock", nil
}

func (e *emptyStateBackend) Unlock(_ context.Context, _, _, _ string) error {
	return nil
}

// loadBackend returns a state backend based on the store block in the loaded
// stack, falling back to the default local backend, then an empty no-op backend.
func loadBackend(loaded *lang.LoadedStack) (core.StateBackend, string) {
	switch loaded.BackendType {
	case "", "local":
		if b, err := state.DefaultLocalBackend(); err == nil {
			return b, "local"
		}
	}
	// Non-local backends are not yet implemented — fall through to empty.
	// Future: wire s3, etcd, postgres backends here.
	return &emptyStateBackend{}, loaded.BackendType
}
