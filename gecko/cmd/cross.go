package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gecko-iac/gecko/internal/cross"
	"github.com/gecko-iac/gecko/internal/ui"
)

var crossCmd = &Command{
	Name:    "cross",
	Short:   "Import Terraform or Pulumi state into Scute",
	Aliases: []string{"bridge"},
	Long: `gecko cross reads a Terraform (.tfstate) or Pulumi stack state file and
generates equivalent Scute declarations, letting you adopt Gecko
incrementally alongside existing IaC tooling.

Resources with known Gecko equivalents are emitted as spawn blocks.
Unmapped resource types (e.g. cloud-provider-specific resources) are
preserved as commented-out blocks so nothing is silently lost.

Like a gecko crossing between territories — bridge two worlds.`,
	Args: []string{},
	Flags: []Flag{
		{Name: "from", Short: "f", Default: "", Usage: "Path to state file (.tfstate or Pulumi stack JSON)", Required: true},
		{Name: "out", Short: "o", Default: "", Usage: "Output .scute file path (default: print to stdout)"},
		{Name: "format", Default: "auto", Usage: "State format: auto, terraform, pulumi"},
		{Name: "project", Short: "p", Default: "imported", Usage: "Project name for the territory block"},
		{Name: "workspace", Short: "w", Default: "default", Usage: "Workspace name for the territory block"},
	},
	Run: runCross,
}

func runCross(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("cross")

	statePath := flagVal(flags, "from", "")
	outPath := flagVal(flags, "out", "")
	format := flagVal(flags, "format", "auto")
	project := flagVal(flags, "project", "imported")
	workspace := flagVal(flags, "workspace", "default")

	if statePath == "" {
		return fmt.Errorf("--from is required: specify a .tfstate or Pulumi state file path")
	}

	// Auto-detect format from path and content
	if format == "auto" {
		format = detectStateFormat(statePath)
		if format == "" {
			return fmt.Errorf(
				"cannot detect state format from %q\nUse --format terraform or --format pulumi",
				statePath,
			)
		}
	}

	ui.Info(fmt.Sprintf("Reading %s state from %s%s%s", format, ui.GeckoTeal, statePath, ui.Reset))
	fmt.Println()

	// Parse
	spin := ui.NewSpinner("Parsing state file")
	spin.Start()

	var (
		result   *cross.CrossResult
		parseErr error
	)
	switch format {
	case "terraform":
		result, parseErr = cross.ParseTerraformState(statePath)
	case "pulumi":
		result, parseErr = cross.ParsePulumiState(statePath)
	default:
		spin.Stop(false)
		return fmt.Errorf("unknown format %q — use terraform or pulumi", format)
	}

	spin.Stop(parseErr == nil)
	if parseErr != nil {
		return fmt.Errorf("failed to parse state: %w", parseErr)
	}

	// Summary
	mapped := result.Mapped()
	unmapped := result.UnmappedResources()

	ui.Label("Total resources", fmt.Sprintf("%d", len(result.Resources)))
	ui.Label("Mapped",
		fmt.Sprintf("%s%d%s", ui.GeckoSuccess, len(mapped), ui.Reset))
	if len(unmapped) > 0 {
		ui.Label("Unmapped",
			fmt.Sprintf("%s%d%s (emitted as comments)", ui.GeckoWarning, len(unmapped), ui.Reset))
	}
	fmt.Println()

	// Warn about unmapped types
	if len(unmapped) > 0 {
		ui.Header("Unmapped resource types")
		seen := make(map[string]bool)
		for _, r := range unmapped {
			if !seen[r.SourceType] {
				seen[r.SourceType] = true
				fmt.Printf("  %s⚠%s  %s%s%s\n",
					ui.GeckoWarning+ui.Bold, ui.Reset,
					ui.GeckoMuted, r.SourceType, ui.Reset)
			}
		}
		fmt.Println()
	}

	if len(mapped) == 0 {
		ui.Warn("No mappable resources found. The generated file will be mostly empty.")
		fmt.Println()
	}

	// Generate Scute
	spin = ui.NewSpinner("Generating Scute declarations")
	spin.Start()
	scute := cross.GenerateScute(result, project, workspace)
	spin.Stop(true)

	// Output
	if outPath == "" {
		fmt.Println()
		fmt.Print(scute)
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(outPath), 0755); err != nil {
		return fmt.Errorf("cannot create output directory: %w", err)
	}
	if err := os.WriteFile(outPath, []byte(scute), 0644); err != nil {
		return fmt.Errorf("cannot write output file: %w", err)
	}

	ui.Success(fmt.Sprintf("Written to %s", outPath))
	fmt.Println()
	fmt.Printf("  %s%sNext steps:%s\n", ui.BrightWhite, ui.Bold, ui.Reset)
	fmt.Printf("    1. Review %s%s%s and configure habitat credentials\n", ui.GeckoTeal, outPath, ui.Reset)
	fmt.Printf("    2. Run %sgecko crawl%s to preview changes\n", ui.GeckoTeal, ui.Reset)
	fmt.Printf("    3. Run %sgecko grip%s to take ownership of these resources\n\n", ui.GeckoTeal, ui.Reset)

	return nil
}

// detectStateFormat tries to determine the state file format from its filename and content.
func detectStateFormat(path string) string {
	// Extension-based
	if strings.ToLower(filepath.Ext(path)) == ".tfstate" {
		return "terraform"
	}

	// Name-based
	base := strings.ToLower(filepath.Base(path))
	if strings.Contains(base, "terraform") {
		return "terraform"
	}
	if strings.Contains(base, "pulumi") {
		return "pulumi"
	}

	// Content-based: peek at a small slice of the file
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	content := string(data)
	if strings.Contains(content, `"terraform_version"`) {
		return "terraform"
	}
	if strings.Contains(content, `"urn:pulumi:`) {
		return "pulumi"
	}

	return ""
}
