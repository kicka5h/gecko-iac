package cmd

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/gecko-iac/gecko/internal/core"
	"github.com/gecko-iac/gecko/internal/lang"
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
	ctx := context.Background()

	ui.PrintBannerSmall("grip")

	workspace := flagVal(flags, "workspace", "dev")
	autoApprove := flagSet(flags, "auto-approve") || flagSet(flags, "y")
	planFile := flagVal(flags, "plan", "")
	target := flagVal(flags, "target", "")

	ui.Info(fmt.Sprintf("Preparing to grip workspace %s%q%s...", ui.GeckoTeal, workspace, ui.Reset))
	fmt.Println()

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
	warnings := registerProviders(ctx, loaded)
	spin.Stop(true)
	for _, w := range warnings {
		ui.Warn(w)
	}

	// 3. Load state backend
	spin = ui.NewSpinner("Loading current state")
	spin.Start()
	stateBackend, _ := loadBackend(loaded)
	spin.Stop(true)

	// 4. Build engine
	engine := core.NewEngine(loaded.Stack, stateBackend)

	// 5. Compute or load plan
	var plan *core.PlanResult
	if planFile != "" {
		spin = ui.NewSpinner(fmt.Sprintf("Loading plan from %s", planFile))
		spin.Start()
		data, err := os.ReadFile(planFile)
		if err != nil {
			spin.Stop(false)
			ui.Error(fmt.Sprintf("Failed to read plan file: %s", err))
			return err
		}
		plan = &core.PlanResult{}
		if err := json.Unmarshal(data, plan); err != nil {
			spin.Stop(false)
			ui.Error(fmt.Sprintf("Failed to parse plan file: %s", err))
			return err
		}
		spin.Stop(true)
	} else {
		spin = ui.NewSpinner("Computing resource dependency graph")
		spin.Start()
		spin.Stop(true)

		spin = ui.NewSpinner("Calculating diffs")
		spin.Start()
		plan, err = engine.Plan(ctx)
		if err != nil {
			spin.Stop(false)
			ui.Error(fmt.Sprintf("Plan failed: %s", err))
			return err
		}
		spin.Stop(true)
	}

	// 6. Filter by target if specified
	if target != "" {
		var filtered []*core.Diff
		for _, d := range plan.Diffs {
			if string(d.ResourceID) == target || strings.HasSuffix(string(d.ResourceID), "::"+target) {
				filtered = append(filtered, d)
			}
		}
		plan.Diffs = filtered
		// Recount
		plan.Adds, plan.Updates, plan.Destroys, plan.NoOps = 0, 0, 0, 0
		for _, d := range plan.Diffs {
			switch d.Kind {
			case core.ChangeAdd:
				plan.Adds++
			case core.ChangeUpdate, core.ChangeReplace:
				plan.Updates++
			case core.ChangeDelete:
				plan.Destroys++
			case core.ChangeNoOp:
				plan.NoOps++
			}
		}
	}

	// 7. Show plan summary
	fmt.Println()
	ui.Header("Changes to Apply")
	fmt.Println()

	hasActual := false
	for _, diff := range plan.Diffs {
		rType, rName := splitResourceID(string(diff.ResourceID))
		switch diff.Kind {
		case core.ChangeAdd:
			ui.PlanAdd(rType, rName)
			hasActual = true
		case core.ChangeUpdate, core.ChangeReplace:
			ui.PlanChange(rType, rName)
			hasActual = true
		case core.ChangeDelete:
			ui.PlanDestroy(rType, rName)
			hasActual = true
		case core.ChangeNoOp:
			ui.PlanNoChange(rType, rName)
		}
	}

	if !hasActual {
		fmt.Println()
		ui.Info("No changes to apply. Infrastructure is up to date.")
		fmt.Println()
		return nil
	}

	fmt.Println()
	ui.PlanSummary(plan.Adds, plan.Updates, plan.Destroys)

	// 8. Confirmation prompt
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

	// 9. Apply with live progress
	type progressEvent struct {
		id     core.ResourceID
		status core.ResourceStatus
	}

	eventCh := make(chan progressEvent, 64)
	spinners := &sync.Map{} // ResourceID → *ui.Spinner

	// Goroutine to render progress events
	renderDone := make(chan struct{})
	go func() {
		defer close(renderDone)
		for ev := range eventCh {
			rType, rName := splitResourceID(string(ev.id))
			label := fmt.Sprintf("%s.%s", rType, rName)
			switch ev.status {
			case core.StatusCreating:
				s := ui.NewSpinner(fmt.Sprintf("  %screating%s  %-50s", ui.GeckoMuted, ui.Reset, label))
				s.Start()
				spinners.Store(ev.id, s)
			case core.StatusUpdating:
				s := ui.NewSpinner(fmt.Sprintf("  %supdating%s  %-50s", ui.GeckoMuted, ui.Reset, label))
				s.Start()
				spinners.Store(ev.id, s)
			case core.StatusRunning:
				if sv, ok := spinners.Load(ev.id); ok {
					sv.(*ui.Spinner).Stop(true)
					spinners.Delete(ev.id)
				}
			case core.StatusFailed:
				if sv, ok := spinners.Load(ev.id); ok {
					sv.(*ui.Spinner).Stop(false)
					spinners.Delete(ev.id)
				}
			}
		}
	}()

	applyStart := time.Now()
	applyResult, applyErr := engine.Apply(ctx, plan, func(id core.ResourceID, status core.ResourceStatus) {
		eventCh <- progressEvent{id: id, status: status}
	})
	close(eventCh)
	<-renderDone

	// Stop any dangling spinners (e.g. on partial failure)
	spinners.Range(func(key, val interface{}) bool {
		val.(*ui.Spinner).Stop(false)
		return true
	})

	elapsed := time.Since(applyStart)
	fmt.Println()
	ui.Divider()
	fmt.Println()

	// 10. Show results
	succeeded := len(applyResult.Succeeded)
	failed := len(applyResult.Failed)

	if failed == 0 {
		fmt.Printf("  %s🦎 Gripped!%s Applied %s%d change(s)%s in %s%.1fs%s\n\n",
			ui.GeckoGreen+ui.Bold, ui.Reset,
			ui.GeckoSuccess+ui.Bold, succeeded, ui.Reset,
			ui.GeckoTeal+ui.Bold, elapsed.Seconds(), ui.Reset)
	} else {
		fmt.Printf("  %s⚠ Partial apply%s — %s%d succeeded%s, %s%d failed%s in %s%.1fs%s\n\n",
			ui.GeckoWarning+ui.Bold, ui.Reset,
			ui.GeckoSuccess, succeeded, ui.Reset,
			ui.GeckoDanger, failed, ui.Reset,
			ui.GeckoTeal, elapsed.Seconds(), ui.Reset)
	}

	if succeeded > 0 {
		ui.Header("Applied Resources")
		ui.TableHeader("Resource", "Status", "Time")
		for _, id := range applyResult.Succeeded {
			rType, rName := splitResourceID(string(id))
			ui.TableRow(fmt.Sprintf("%s.%s", rType, rName), ui.StatusTag("running"), "-")
		}
		fmt.Println()
	}

	if failed > 0 {
		ui.Header("Failed Resources")
		for _, id := range applyResult.Failed {
			rType, rName := splitResourceID(string(id))
			ui.Error(fmt.Sprintf("%s.%s failed to apply", rType, rName))
		}
		fmt.Println()
	}

	ui.Info("Run 'gecko bask' to view your infrastructure dashboard.")
	ui.Info("Run 'gecko tail' to stream live logs.")
	fmt.Println()

	if applyErr != nil {
		return applyErr
	}
	return nil
}
