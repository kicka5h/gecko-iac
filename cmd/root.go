// Package cmd contains all Gecko CLI commands.
// Commands are intentionally named after gecko anatomy and behaviors.
package cmd

import (
	"fmt"
	"os"

	"github.com/gecko-iac/gecko/internal/ui"
)

// CLI version
const Version = "0.1.0"

// Command represents a Gecko CLI subcommand
type Command struct {
	Name        string
	Short       string
	Long        string
	Aliases     []string
	Flags       []Flag
	Args        []string
	Run         func(args []string, flags map[string]string) error
	Subcommands map[string]*Command
}

// Flag represents a CLI flag
type Flag struct {
	Name     string
	Short    string
	Default  string
	Usage    string
	Required bool
}

// rootCmd is the top-level command registry
var rootCmd = &Command{
	Name:  "gecko",
	Short: "🦎 Gecko — FOSS Infrastructure as Code",
	Long: `Gecko is an open-source Infrastructure as Code framework that prioritizes
FOSS infrastructure solutions. Manage Kubernetes, Proxmox, Nomad, Gitea,
and more with a unified, elegant workflow.`,
}

// allCommands registers every gecko subcommand
var allCommands = map[string]*Command{}

// register adds a command to the registry
func register(cmd *Command) {
	allCommands[cmd.Name] = cmd
	for _, alias := range cmd.Aliases {
		allCommands[alias] = cmd
	}
}

// Execute is the main entry point for the CLI
func Execute() error {
	args := os.Args[1:]

	if len(args) == 0 {
		ui.PrintBanner()
		printHelp()
		return nil
	}

	// Handle global flags
	if args[0] == "--version" || args[0] == "-v" {
		fmt.Printf("🦎 gecko v%s\n", Version)
		return nil
	}

	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		if len(args) > 1 {
			if cmd, ok := allCommands[args[1]]; ok {
				printCommandHelp(cmd)
				return nil
			}
		}
		ui.PrintBanner()
		printHelp()
		return nil
	}

	cmdName := args[0]
	cmd, ok := allCommands[cmdName]
	if !ok {
		ui.PrintBannerSmall("error")
		ui.Error(fmt.Sprintf("Unknown command: %q", cmdName))
		fmt.Println()
		ui.Info("Run 'gecko help' to see available commands.")
		fmt.Println()
		return fmt.Errorf("unknown command: %s", cmdName)
	}

	// Parse flags for this command
	remaining, flags, err := parseFlags(args[1:], cmd.Flags)
	if err != nil {
		ui.Error(err.Error())
		return err
	}

	if flagSet(flags, "help") || flagSet(flags, "h") {
		printCommandHelp(cmd)
		return nil
	}

	return cmd.Run(remaining, flags)
}

// parseFlags parses --flag=value and --flag value style args
func parseFlags(args []string, defs []Flag) (remaining []string, flags map[string]string, err error) {
	flags = make(map[string]string)

	// Set defaults
	for _, def := range defs {
		if def.Default != "" {
			flags[def.Name] = def.Default
		}
	}

	i := 0
	for i < len(args) {
		arg := args[i]
		if len(arg) >= 2 && arg[0] == '-' {
			// Strip leading dashes
			stripped := arg
			if len(arg) >= 2 && arg[1] == '-' {
				stripped = arg[2:]
			} else {
				stripped = arg[1:]
			}

			// Check for = separator
			eq := -1
			for j, c := range stripped {
				if c == '=' {
					eq = j
					break
				}
			}

			var name, value string
			if eq >= 0 {
				name = stripped[:eq]
				value = stripped[eq+1:]
			} else {
				name = stripped
				// Check if next arg is the value
				if i+1 < len(args) && len(args[i+1]) > 0 && args[i+1][0] != '-' {
					i++
					value = args[i]
				} else {
					value = "true"
				}
			}

			// Resolve short name to long name
			for _, def := range defs {
				if def.Short == name {
					name = def.Name
					break
				}
			}

			flags[name] = value
		} else {
			remaining = append(remaining, arg)
		}
		i++
	}

	// Validate required flags
	for _, def := range defs {
		if def.Required {
			if _, ok := flags[def.Name]; !ok {
				return nil, nil, fmt.Errorf("required flag --%s is missing", def.Name)
			}
		}
	}

	return remaining, flags, nil
}

func flagSet(flags map[string]string, name string) bool {
	v, ok := flags[name]
	return ok && v != "" && v != "false"
}

func flagVal(flags map[string]string, name, def string) string {
	if v, ok := flags[name]; ok && v != "" {
		return v
	}
	return def
}

// printHelp prints the top-level help screen
func printHelp() {
	fmt.Printf("  %sUsage:%s gecko <command> [flags]\n\n", ui.BrightWhite+ui.Bold, ui.Reset)

	fmt.Printf("  %s%sScaling Commands%s%s\n", ui.GeckoGreen, ui.Bold, ui.Reset, ui.Reset)
	printCmdLine("hatch",  "Initialize a new Gecko project                 [🥚 new project]")
	printCmdLine("crawl",  "Preview infrastructure changes (plan)           [🦶 careful steps]")
	printCmdLine("grip",   "Apply changes to infrastructure                 [🤲 lock in]")
	printCmdLine("molt",   "Destroy all managed infrastructure               [💀 shed everything]")
	fmt.Println()

	fmt.Printf("  %s%sObservability Commands%s%s\n", ui.GeckoGreen, ui.Bold, ui.Reset, ui.Reset)
	printCmdLine("bask",   "Show infrastructure status dashboard            [☀️  soak it in]")
	printCmdLine("tail",   "Stream live logs from infrastructure            [🔍 follow the tail]")
	printCmdLine("lick",   "Inspect a specific resource in detail           [👅 taste test]")
	fmt.Println()

	fmt.Printf("  %s%sManagement Commands%s%s\n", ui.GeckoGreen, ui.Bold, ui.Reset, ui.Reset)
	printCmdLine("shed",   "Upgrade providers and framework                 [🐍 new skin]")
	printCmdLine("hide",   "Manage secrets and sensitive values             [🫙 keep it secret]")
	printCmdLine("burrow", "Import existing resources into Gecko state      [🕳️  dig in]")
	printCmdLine("scale",  "Scale a resource up or down                     [📊 adjust]")
	printCmdLine("clutch", "Manage stacks (create/select/list)              [🥚 egg clutch]")
	fmt.Println()

	fmt.Printf("  %s%sOther%s%s\n", ui.GeckoGreen, ui.Bold, ui.Reset, ui.Reset)
	printCmdLine("snack",   "Manage Snack modules — reusable infra building blocks [🍽  snack]")
	printCmdLine("cross",   "Import Terraform or Pulumi state into Scute       [🌉 bridge]")
	printCmdLine("check",   "Validate .scute files (parse + type check)       [🔬 inspect]")
	printCmdLine("fmt",     "Format .scute files in canonical style            [✨ clean]")
	printCmdLine("run",     "Evaluate a .scute file and show resolved graph   [▶ dry eval]")
	printCmdLine("help",    "Show help for any command")
	printCmdLine("version", "Show gecko version information")
	fmt.Println()

	fmt.Printf("  %sRun %s gecko <command> --help %sfor command-specific help.%s\n\n",
		ui.GeckoMuted, ui.GeckoTeal, ui.GeckoMuted, ui.Reset)
}

func printCmdLine(name, desc string) {
	fmt.Printf("    %sgecko %-10s%s  %s%s%s\n",
		ui.GeckoTeal+ui.Bold, name, ui.Reset,
		ui.GeckoMuted, desc, ui.Reset)
}

func printCommandHelp(cmd *Command) {
	ui.PrintBannerSmall(cmd.Name)
	fmt.Printf("  %s%s%s\n\n", ui.BrightWhite, cmd.Long, ui.Reset)
	fmt.Printf("  %sUsage:%s gecko %s [flags]", ui.Bold, ui.Reset, cmd.Name)
	if len(cmd.Args) > 0 {
		for _, a := range cmd.Args {
			fmt.Printf(" <%s>", a)
		}
	}
	fmt.Println()

	if len(cmd.Aliases) > 0 {
		fmt.Printf("\n  %sAliases:%s %s\n", ui.Bold, ui.Reset, fmt.Sprint(cmd.Aliases))
	}

	if len(cmd.Flags) > 0 {
		fmt.Printf("\n  %sFlags:%s\n", ui.Bold, ui.Reset)
		for _, f := range cmd.Flags {
			short := ""
			if f.Short != "" {
				short = fmt.Sprintf("-%s, ", f.Short)
			}
			def := ""
			if f.Default != "" {
				def = fmt.Sprintf(" (default: %s)", f.Default)
			}
			req := ""
			if f.Required {
				req = " " + ui.GeckoDanger + "[required]" + ui.Reset
			}
			fmt.Printf("    %s%s--%s%s%s  %s%s%s%s\n",
				ui.GeckoMuted, short, f.Name, ui.Reset,
				ui.GeckoTeal, f.Usage, ui.GeckoMuted, def+req, ui.Reset)
		}
	}
	fmt.Println()
}

func init() {
	register(hatchCmd)
	register(crawlCmd)
	register(gripCmd)
	register(moltCmd)
	register(baskCmd)
	register(tailCmd)
	register(lickCmd)
	register(shedCmd)
	register(hideCmd)
	register(burrowCmd)
	register(scaleCmd)
	register(clutchCmd)
	register(snackCmd)
	register(checkCmd)
	register(fmtCmd)
	register(runCmd)
	register(crossCmd)

	register(&Command{
		Name:  "version",
		Short: "Show gecko version",
		Long:  "Display the current Gecko version and build information.",
		Run: func(args []string, flags map[string]string) error {
			ui.PrintBannerSmall("version")
			ui.Label("Version", Version)
			ui.Label("Runtime", "Go 1.22")
			ui.Label("License", "Apache 2.0")
			ui.Label("Homepage", "https://gecko-iac.dev")
			fmt.Println()
			return nil
		},
	})
}
