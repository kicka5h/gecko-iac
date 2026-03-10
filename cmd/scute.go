package cmd

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/gecko-iac/gecko/internal/lang"
	"github.com/gecko-iac/gecko/internal/ui"
)

var checkCmd = &Command{
	Name:    "check",
	Short:   "Parse and validate .scute files without touching infrastructure",
	Aliases: []string{"validate", "lint"},
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "dev", Usage: "Target workspace"},
		{Name: "dir", Short: "d", Default: ".", Usage: "Project directory"},
	},
	Run: runCheck,
}

func runCheck(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("check")
	workspace := flagVal(flags, "workspace", "dev")
	dir := flagVal(flags, "dir", ".")
	absDir, _ := filepath.Abs(dir)
	result := lang.Check(absDir, workspace)

	if result.Stats.Files == 0 {
		ui.Warn("No .scute files found.")
		ui.Indent(fmt.Sprintf("Looked in: %s/stacks/%s/ and %s/", absDir, workspace, absDir))
		fmt.Println()
		return nil
	}

	ui.Header("Scute Project")
	ui.Label("Directory", absDir)
	ui.Label("Workspace", workspace)
	ui.Label("Files", fmt.Sprintf("%d", result.Stats.Files))
	fmt.Println()

	if len(result.Errors) > 0 {
		ui.Header(fmt.Sprintf("✗ %d Error(s)", len(result.Errors)))
		for _, err := range result.Errors {
			fmt.Printf("  %s✗%s %s\n", ui.GeckoDanger+ui.Bold, ui.Reset, err.Error())
		}
		fmt.Println()
		return fmt.Errorf("%d error(s) found", len(result.Errors))
	}

	ui.Header("Declarations")
	ui.Label("Spawns   (resources)", fmt.Sprintf("%d", result.Stats.Spawns))
	ui.Label("Habitats (providers)", fmt.Sprintf("%d", result.Stats.Habitats))
	ui.Label("Marks    (variables)", fmt.Sprintf("%d", result.Stats.Marks))
	ui.Label("Signals  (outputs)", fmt.Sprintf("%d", result.Stats.Signals))
	fmt.Println()
	ui.Success("All .scute files are valid. Ready to crawl.")
	fmt.Println()
	return nil
}

var fmtCmd = &Command{
	Name:    "fmt",
	Short:   "Format .scute files in canonical Scute style",
	Aliases: []string{"format"},
	Flags: []Flag{
		{Name: "check", Short: "c", Default: "false", Usage: "Only check, don't write"},
		{Name: "dir", Short: "d", Default: ".", Usage: "Directory to format"},
	},
	Run: runFmt,
}

func runFmt(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("fmt")
	checkOnly := flagSet(flags, "check")
	dir := flagVal(flags, "dir", ".")
	var scuteFiles []string
	if len(args) > 0 {
		scuteFiles = args
	} else {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil { return err }
			if !info.IsDir() && strings.HasSuffix(path, ".scute") {
				scuteFiles = append(scuteFiles, path)
			}
			return nil
		})
	}
	if len(scuteFiles) == 0 {
		ui.Warn("No .scute files found.")
		fmt.Println()
		return nil
	}
	changed := 0
	for _, f := range scuteFiles {
		src, err := os.ReadFile(f)
		if err != nil { ui.Error(fmt.Sprintf("Cannot read %s: %v", f, err)); continue }
		formatted, errs := lang.Format(string(src), f)
		if len(errs) > 0 {
			for _, e := range errs { ui.Error(fmt.Sprintf("%s: %v", f, e)) }
			continue
		}
		if string(src) != formatted {
			changed++
			if checkOnly {
				fmt.Printf("  %s≠%s %s %s(would reformat)%s\n", ui.GeckoWarning+ui.Bold, ui.Reset, f, ui.GeckoMuted, ui.Reset)
			} else {
				if err := os.WriteFile(f, []byte(formatted), 0644); err != nil {
					ui.Error(fmt.Sprintf("Cannot write %s: %v", f, err))
				} else {
					fmt.Printf("  %s✎%s %s\n", ui.GeckoTeal+ui.Bold, ui.Reset, f)
				}
			}
		} else {
			fmt.Printf("  %s✓%s %s %s(canonical)%s\n", ui.GeckoSuccess+ui.Bold, ui.Reset, f, ui.GeckoMuted, ui.Reset)
		}
	}
	fmt.Println()
	if checkOnly && changed > 0 {
		return fmt.Errorf("%d file(s) need formatting", changed)
	}
	if changed > 0 {
		ui.Success(fmt.Sprintf("Formatted %d file(s).", changed))
	} else {
		ui.Success("All files are already canonical.")
	}
	fmt.Println()
	return nil
}

var runCmd = &Command{
	Name:  "run",
	Short: "Evaluate a .scute file and show resolved declarations",
	Flags: []Flag{
		{Name: "workspace", Short: "w", Default: "dev", Usage: "Workspace name"},
	},
	Run: runRun,
}

func runRun(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("run")
	file := ""
	if len(args) > 0 {
		file = args[0]
	} else {
		// Walk current directory tree looking for any .scute file
		_ = filepath.WalkDir(".", func(path string, d fs.DirEntry, err error) error {
			if err != nil || file != "" {
				return nil
			}
			if !d.IsDir() && strings.HasSuffix(d.Name(), ".scute") {
				file = path
			}
			return nil
		})
	}
	if file == "" {
		ui.Error("No .scute file specified and no main.scute found.")
		return fmt.Errorf("no file")
	}
	src, err := os.ReadFile(file)
	if err != nil { return err }

	ui.Label("File", file)
	fmt.Println()

	prog, errs := lang.ParseFile(string(src), file)
	if len(errs) > 0 {
		ui.Header("Parse Errors")
		for _, e := range errs { ui.Error(e.Error()) }
		return fmt.Errorf("parse failed")
	}

	eval := lang.NewEvaluator(file)
	result, evalErrs := eval.Eval(prog)
	if len(evalErrs) > 0 {
		ui.Header("Evaluation Errors")
		for _, e := range evalErrs { ui.Error(e.Error()) }
	}

	if result.ProjectName != "" {
		ui.Header("Territory")
		ui.Label("Name", result.ProjectName)
		ui.Label("Workspace", result.Workspace)
		fmt.Println()
	}
	if len(result.Providers) > 0 {
		ui.Header(fmt.Sprintf("Habitats (%d)", len(result.Providers)))
		for _, p := range result.Providers {
			fmt.Printf("  %s%s%s\n", ui.GeckoTeal+ui.Bold, p.Name, ui.Reset)
			for k, v := range p.Config { ui.Label("    "+k, fmt.Sprintf("%v", v)) }
		}
		fmt.Println()
	}
	if len(result.Vars) > 0 {
		ui.Header(fmt.Sprintf("Marks (%d)", len(result.Vars)))
		for k, v := range result.Vars { ui.Label("  "+k, v.String()) }
		fmt.Println()
	}
	if len(result.Resources) > 0 {
		ui.Header(fmt.Sprintf("Spawns (%d)", len(result.Resources)))
		fmt.Println()
		for _, rr := range result.Resources {
			prot := ""
			if rr.Protected {
				prot = fmt.Sprintf(" %s🛡 protected%s", ui.GeckoWarning, ui.Reset)
			}
			fmt.Printf("  %s%s%s  %s%s%s%s\n",
				ui.GeckoGreen+ui.Bold, rr.TypeStr, ui.Reset,
				ui.BrightWhite+ui.Bold, rr.Name, ui.Reset, prot)
			for k, v := range rr.Args.Inputs {
				fmt.Printf("    %s%-24s%s %v\n", ui.GeckoMuted, k, ui.Reset, v)
			}
			if len(rr.DependsOn) > 0 {
				fmt.Printf("    %sneeds%s                    ", ui.GeckoMuted, ui.Reset)
				for _, dep := range rr.DependsOn {
					fmt.Printf("%s@%s%s ", ui.GeckoGreen, dep, ui.Reset)
				}
				fmt.Println()
			}
			fmt.Println()
		}
	}
	if len(result.Outputs) > 0 {
		ui.Header(fmt.Sprintf("Signals (%d)", len(result.Outputs)))
		for k, v := range result.Outputs { ui.Label("  "+k, v.String()) }
		fmt.Println()
	}
	return nil
}
