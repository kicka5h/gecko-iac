package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gecko-iac/gecko/internal/lang"
	"github.com/gecko-iac/gecko/internal/ui"
)

// snackCmd is the top-level `gecko snack` command group
var snackCmd = &Command{
	Name:    "snack",
	Short:   "Manage Snack modules — Gecko's reusable infrastructure building blocks",
	Aliases: []string{},
	Long: `Snacks are reusable, parameterized infrastructure modules. A .snack file
exports named templates of spawn blocks that any .scute file can consume:

  snack "monitoring" from "./modules/monitoring.snack"
    namespace: "observability"
    replicas:  3
  end

Sources can be local paths, HTTPS URLs, Gitea repos, GitHub repos, or the
Gecko snack registry (snack:org/name@version).

Subcommands:
  gecko snack add <source>    — fetch and cache a snack
  gecko snack list            — list cached snacks
  gecko snack update [name]   — refresh cached snacks
  gecko snack new <name>      — scaffold a new .snack file
  gecko snack info <source>   — show params, emits, and spawns
  gecko snack verify [name]   — check snack integrity`,
	Flags: []Flag{},
	Run:   runSnackHelp,
	Subcommands: map[string]*Command{
		"add":    snackAddCmd,
		"list":   snackListCmd,
		"update": snackUpdateCmd,
		"new":    snackNewCmd,
		"info":   snackInfoCmd,
		"verify": snackVerifyCmd,
	},
}

func runSnackHelp(args []string, flags map[string]string) error {
	// Route to subcommand if first arg matches
	if len(args) > 0 {
		subs := map[string]*Command{
			"add":    snackAddCmd,
			"list":   snackListCmd,
			"update": snackUpdateCmd,
			"new":    snackNewCmd,
			"info":   snackInfoCmd,
			"verify": snackVerifyCmd,
		}
		if sub, ok := subs[args[0]]; ok {
			remaining, parsedFlags, _ := parseFlags(args[1:], sub.Flags)
			return sub.Run(remaining, parsedFlags)
		}
	}
	ui.PrintBannerSmall("snack")
	fmt.Printf(`Snacks are reusable, parameterized infrastructure modules.

  gecko snack add <source>    fetch and cache a snack
  gecko snack list            list cached snacks
  gecko snack update          refresh cached snacks
  gecko snack new <name>      scaffold a new .snack file
  gecko snack info <source>   show params, emits, and spawns
  gecko snack verify [name]   check snack integrity
`)
	fmt.Println()
	return nil
}

// ─── gecko snack add ──────────────────────────────────────────────────────────

var snackAddCmd = &Command{
	Name:  "add",
	Short: "Fetch and cache a snack from a URL, Gitea repo, or registry",
	Long: `gecko snack add <source>

Fetches a .snack file and stores it in ~/.gecko/snacks/ for offline use.
Sources:
  ./path/to/module.snack             local file (always resolved fresh)
  https://example.com/module.snack   direct HTTPS URL
  gitea://host/owner/repo/file@ref   Gitea raw file
  github://owner/repo/path@ref       GitHub raw file
  snack:org/name@v1.0.0              Gecko snack registry`,
	Flags: []Flag{
		{Name: "force", Short: "f", Default: "false", Usage: "Re-fetch even if cached"},
	},
	Run: runSnackAdd,
}

func runSnackAdd(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("snack add")

	if len(args) == 0 {
		ui.Error("Usage: gecko snack add <source>")
		return fmt.Errorf("source required")
	}

	source := args[0]
	cacheDir := lang.CacheDir()
	cwd, _ := os.Getwd()

	sp := ui.NewSpinner(fmt.Sprintf("Fetching %s", source))
	sp.Start()

	data, src, err := lang.FetchSnack(source, cwd, cacheDir)
	sp.Stop(true)

	if err != nil {
		ui.Error(fmt.Sprintf("Failed to fetch snack: %v", err))
		return err
	}

	// Parse to validate and extract export names
	sf, parseErrs := lang.ParseSnackFile(string(data), source)
	if len(parseErrs) > 0 {
		ui.Warn("Snack fetched but has parse errors:")
		for _, e := range parseErrs {
			fmt.Printf("  %s\n", e.Error())
		}
	}

	ui.Success(fmt.Sprintf("Snack cached from %s (%s)", src.Kind, source))
	fmt.Println()

	if sf != nil && len(sf.Exports) > 0 {
		ui.Header(fmt.Sprintf("Exports (%d)", len(sf.Exports)))
		for _, ex := range sf.Exports {
			fmt.Printf("  %s%s%s", ui.GeckoGreen+ui.Bold, ex.Name, ui.Reset)
			if len(ex.Params) > 0 {
				params := make([]string, len(ex.Params))
				for i, p := range ex.Params {
					params[i] = p.Name
				}
				fmt.Printf("  %s(%s)%s", ui.GeckoMuted, strings.Join(params, ", "), ui.Reset)
			}
			fmt.Println()
			for _, s := range ex.Spawns {
				fmt.Printf("    %s+ %s%s  %s\n", ui.GeckoTeal, s.TypeStr, ui.Reset, s.Name)
			}
			for _, em := range ex.Emits {
				fmt.Printf("    %s↑ emit %s%s\n", ui.GeckoWarning, em.Name, ui.Reset)
			}
		}
		fmt.Println()
	}

	return nil
}

// ─── gecko snack list ─────────────────────────────────────────────────────────

var snackListCmd = &Command{
	Name:  "list",
	Short: "List cached snack modules",
	Flags: []Flag{},
	Run:   runSnackList,
}

func runSnackList(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("snack list")
	cacheDir := lang.CacheDir()
	infos, err := lang.ListCachedSnacks(cacheDir)
	if err != nil {
		ui.Error(fmt.Sprintf("Cannot read snack cache: %v", err))
		return err
	}

	// Also scan project directory for local .snack files
	cwd, _ := os.Getwd()
	var localSnacks []string
	_ = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
		if err != nil { return nil }
		if !info.IsDir() && strings.HasSuffix(path, ".snack") {
			rel, _ := filepath.Rel(cwd, path)
			localSnacks = append(localSnacks, rel)
		}
		return nil
	})

	if len(localSnacks) > 0 {
		ui.Header(fmt.Sprintf("Local Snacks (%d)", len(localSnacks)))
		for _, f := range localSnacks {
			fmt.Printf("  %s📦%s %s\n", ui.GeckoGreen, ui.Reset, f)
		}
		fmt.Println()
	}

	if len(infos) == 0 {
		ui.Header("Cached Snacks")
		ui.Indent("No snacks cached. Use: gecko snack add <source>")
		fmt.Println()
		return nil
	}

	ui.Header(fmt.Sprintf("Cached Snacks (%d)", len(infos)))
	ui.TableHeader("Name", "Size", "Cached")
	for _, info := range infos {
		age := formatAge(time.Since(info.CachedAt))
		ui.TableRow(info.Name, fmt.Sprintf("%d B", info.Size), age)
	}
	fmt.Println()
	ui.Indent(fmt.Sprintf("Cache location: %s", filepath.Join(cacheDir, "snacks")))
	fmt.Println()
	return nil
}

// ─── gecko snack update ───────────────────────────────────────────────────────

var snackUpdateCmd = &Command{
	Name:  "update",
	Short: "Re-fetch cached remote snacks to get latest versions",
	Flags: []Flag{
		{Name: "all", Short: "a", Default: "false", Usage: "Update all cached snacks"},
	},
	Run: runSnackUpdate,
}

func runSnackUpdate(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("snack update")
	cacheDir := lang.CacheDir()
	snackDir := filepath.Join(cacheDir, "snacks")

	entries, err := os.ReadDir(snackDir)
	if err != nil {
		if os.IsNotExist(err) {
			ui.Info("No cached snacks to update.")
			return nil
		}
		return err
	}

	updated := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".snack") {
			continue
		}
		// Delete cached file to force re-fetch on next use
		path := filepath.Join(snackDir, e.Name())
		if err := os.Remove(path); err == nil {
			fmt.Printf("  %s↺%s %s\n", ui.GeckoTeal+ui.Bold, ui.Reset, e.Name())
			updated++
		}
	}

	fmt.Println()
	if updated == 0 {
		ui.Info("No cached remote snacks found.")
	} else {
		ui.Success(fmt.Sprintf("Cleared %d cached snack(s) — they will be re-fetched on next use.", updated))
	}
	fmt.Println()
	return nil
}

// ─── gecko snack new ──────────────────────────────────────────────────────────

var snackNewCmd = &Command{
	Name:  "new",
	Short: "Scaffold a new .snack file",
	Flags: []Flag{
		{Name: "dir", Short: "d", Default: ".", Usage: "Directory to create snack in"},
	},
	Run: runSnackNew,
}

func runSnackNew(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("snack new")

	name := "my-module"
	if len(args) > 0 {
		name = args[0]
	}
	dir := flagVal(flags, "dir", ".")

	filename := filepath.Join(dir, name+".snack")
	if _, err := os.Stat(filename); err == nil {
		ui.Error(fmt.Sprintf("%s already exists", filename))
		return fmt.Errorf("file exists")
	}

	content := generateSnackTemplate(name)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
		return err
	}

	ui.Success(fmt.Sprintf("Scaffolded: %s", filename))
	fmt.Println()
	ui.Header("Usage in your .scute file")
	fmt.Printf("  %ssnack %q from \"./%s\"%s\n", ui.GeckoTeal, name, name+".snack", ui.Reset)
	fmt.Printf("  %s  param_name: value%s\n", ui.GeckoMuted, ui.Reset)
	fmt.Printf("  %send%s\n", ui.GeckoTeal, ui.Reset)
	fmt.Println()
	return nil
}

// ─── gecko snack info ─────────────────────────────────────────────────────────

var snackInfoCmd = &Command{
	Name:  "info",
	Short: "Show parameters, emits, and spawns of a snack",
	Flags: []Flag{},
	Run:   runSnackInfo,
}

func runSnackInfo(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("snack info")

	if len(args) == 0 {
		ui.Error("Usage: gecko snack info <source>")
		return fmt.Errorf("source required")
	}

	source := args[0]
	cacheDir := lang.CacheDir()
	cwd, _ := os.Getwd()

	data, src, err := lang.FetchSnack(source, cwd, cacheDir)
	if err != nil {
		ui.Error(fmt.Sprintf("Cannot fetch snack: %v", err))
		return err
	}

	sf, parseErrs := lang.ParseSnackFile(string(data), source)
	if len(parseErrs) > 0 {
		ui.Header("Parse Errors")
		for _, e := range parseErrs {
			ui.Error(e.Error())
		}
		return fmt.Errorf("parse failed")
	}

	ui.Label("Source", source)
	ui.Label("Kind", src.Kind)
	ui.Label("Exports", fmt.Sprintf("%d", len(sf.Exports)))
	fmt.Println()

	for _, ex := range sf.Exports {
		fmt.Printf("  %s%s%s\n", ui.GeckoGreen+ui.Bold+ui.BrightWhite, ex.Name, ui.Reset)
		fmt.Println()

		if len(ex.Params) > 0 {
			ui.Header("  Parameters")
			for _, p := range ex.Params {
				typeStr := tokenTypeStr(p.TypeHint)
				defStr := ""
				if p.Default != nil {
					defStr = fmt.Sprintf("  %s(default)%s", ui.GeckoMuted, ui.Reset)
				} else {
					defStr = fmt.Sprintf("  %s(required)%s", ui.GeckoDanger, ui.Reset)
				}
				fmt.Printf("    %s%-20s%s %s%-8s%s%s\n",
					ui.BrightWhite, p.Name, ui.Reset,
					ui.GeckoTeal, typeStr, ui.Reset,
					defStr)
			}
			fmt.Println()
		}

		if len(ex.Spawns) > 0 {
			ui.Header("  Spawns")
			for _, s := range ex.Spawns {
				prot := ""
				if s.Protected {
					prot = " 🛡"
				}
				fmt.Printf("    %s+ %s%s  as  %s%s%s%s\n",
					ui.GeckoGreen, s.TypeStr, ui.Reset,
					ui.BrightWhite, s.Name, ui.Reset, prot)
			}
			fmt.Println()
		}

		if len(ex.Emits) > 0 {
			ui.Header("  Emits")
			for _, em := range ex.Emits {
				fmt.Printf("    %s↑ %s%s\n", ui.GeckoWarning+ui.Bold, em.Name, ui.Reset)
			}
			fmt.Println()
		}
	}

	return nil
}

// ─── gecko snack verify ───────────────────────────────────────────────────────

var snackVerifyCmd = &Command{
	Name:  "verify",
	Short: "Parse and validate .snack files to check they are well-formed",
	Flags: []Flag{},
	Run:   runSnackVerify,
}

func runSnackVerify(args []string, flags map[string]string) error {
	ui.PrintBannerSmall("snack verify")

	sources := args
	if len(sources) == 0 {
		// Default: verify all local .snack files
		cwd, _ := os.Getwd()
		_ = filepath.Walk(cwd, func(path string, info os.FileInfo, err error) error {
			if err != nil { return nil }
			if !info.IsDir() && strings.HasSuffix(path, ".snack") {
				sources = append(sources, path)
			}
			return nil
		})
	}

	if len(sources) == 0 {
		ui.Info("No .snack files found in current directory.")
		return nil
	}

	allOk := true
	for _, source := range sources {
		data, err := os.ReadFile(source)
		if err != nil {
			fmt.Printf("  %s✗%s %s — cannot read: %v\n", ui.GeckoDanger+ui.Bold, ui.Reset, source, err)
			allOk = false
			continue
		}
		_, parseErrs := lang.ParseSnackFile(string(data), source)
		if len(parseErrs) > 0 {
			fmt.Printf("  %s✗%s %s\n", ui.GeckoDanger+ui.Bold, ui.Reset, source)
			for _, e := range parseErrs {
				fmt.Printf("      %s\n", e.Error())
			}
			allOk = false
		} else {
			fmt.Printf("  %s✓%s %s\n", ui.GeckoSuccess+ui.Bold, ui.Reset, source)
		}
	}

	fmt.Println()
	if allOk {
		ui.Success("All snacks are valid.")
	} else {
		ui.Error("Some snacks have errors.")
		return fmt.Errorf("validation failed")
	}
	fmt.Println()
	return nil
}

// ─── Template Generator ───────────────────────────────────────────────────────

func generateSnackTemplate(name string) string {
	return fmt.Sprintf(`# %s.snack — Gecko Snack Module
#
# A snack is a reusable, parameterized bundle of spawn blocks.
# Consume this snack in any .scute file:
#
#   snack "%s" from "./%s.snack"
#     namespace: "my-namespace"
#     replicas:  3
#   end
#
# Then access emitted values:
#   signal "endpoint"
#     value: %s.endpoint
#   end

export "%s"
  # ── Parameters ───────────────────────────────────────────────────────────────
  # Parameters let callers customize this module.
  # Declare types and defaults; callers override in their snack block.
  param namespace  string: "%s"
  param replicas   number: 2
  param image      string: "nginx:latest"
  param port       number: 8080

  # ── Computed locals ───────────────────────────────────────────────────────────
  camouflage
    labels:
      app:        "%s"
      managed-by: "gecko"
    end
  end

  # ── Resources this snack spawns ───────────────────────────────────────────────
  spawn "proxmox:vm" as "web"
    node:   "pve"
    name:   namespace
    cores:  2
    memory: 4096
    clone:  image
    tags:   labels
  end

  spawn "proxmox:network" as "net"
    needs: @web
    node:  "pve"
    iface: "vmbr1"
    cidr:  "10.10.10.1/24"
  end

  spawn "fly:app" as "app"
    name:   namespace
    org:    "personal"
  end

  # ── Emitted values (accessible as %s.signal_name) ─────────────────────────────
  emit "namespace"
    value: @ns.name
  end

  emit "endpoint"
    value: "http://#{namespace}.svc.cluster.local:#{port}"
  end

  emit "service_name"
    value: @svc.name
  end
end
`, name, name, name, name, name, name, name, name)
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func formatAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func tokenTypeStr(tt lang.TokenType) string {
	switch tt {
	case lang.TYPE_STRING:
		return "string"
	case lang.TYPE_NUMBER:
		return "number"
	case lang.TYPE_BOOL:
		return "bool"
	case lang.TYPE_LIST:
		return "list"
	case lang.TYPE_MAP:
		return "map"
	case lang.TYPE_ANY:
		return "any"
	}
	return "any"
}

func init() {
	snackCmd.Subcommands = map[string]*Command{
		"add":    snackAddCmd,
		"list":   snackListCmd,
		"update": snackUpdateCmd,
		"new":    snackNewCmd,
		"info":   snackInfoCmd,
		"verify": snackVerifyCmd,
	}
}
