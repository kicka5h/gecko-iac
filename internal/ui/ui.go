package ui

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

// ANSI color codes
const (
	Reset     = "\033[0m"
	Bold      = "\033[1m"
	Dim       = "\033[2m"
	Italic    = "\033[3m"
	Underline = "\033[4m"

	// Foreground colors
	Black   = "\033[30m"
	Red     = "\033[31m"
	Green   = "\033[32m"
	Yellow  = "\033[33m"
	Blue    = "\033[34m"
	Magenta = "\033[35m"
	Cyan    = "\033[36m"
	White   = "\033[37m"

	// Bright foreground
	BrightBlack   = "\033[90m"
	BrightRed     = "\033[91m"
	BrightGreen   = "\033[92m"
	BrightYellow  = "\033[93m"
	BrightBlue    = "\033[94m"
	BrightMagenta = "\033[95m"
	BrightCyan    = "\033[96m"
	BrightWhite   = "\033[97m"

	// Background colors
	BgBlack   = "\033[40m"
	BgGreen   = "\033[42m"
	BgYellow  = "\033[43m"
	BgBlue    = "\033[44m"
	BgMagenta = "\033[45m"
	BgCyan    = "\033[46m"

	// Gecko brand colors (256-color)
	GeckoGreen   = "\033[38;5;82m"
	GeckoTeal    = "\033[38;5;43m"
	GeckoLime    = "\033[38;5;118m"
	GeckoEarth   = "\033[38;5;136m"
	GeckoShadow  = "\033[38;5;238m"
	GeckoMuted   = "\033[38;5;245m"
	GeckoDark    = "\033[38;5;232m"
	GeckoWarning = "\033[38;5;220m"
	GeckoDanger  = "\033[38;5;196m"
	GeckoSuccess = "\033[38;5;46m"
	GeckoInfo    = "\033[38;5;39m"

	BgGeckoGreen = "\033[48;5;22m"
	BgGeckoDark  = "\033[48;5;232m"
)

// GeckoAscii is the main brand banner
const GeckoAscii = `
   ██████╗ ███████╗ ██████╗██╗  ██╗ ██████╗ 
  ██╔════╝ ██╔════╝██╔════╝██║ ██╔╝██╔═══██╗
  ██║  ███╗█████╗  ██║     █████╔╝ ██║   ██║
  ██║   ██║██╔══╝  ██║     ██╔═██╗ ██║   ██║
  ╚██████╔╝███████╗╚██████╗██║  ██╗╚██████╔╝
   ╚═════╝ ╚══════╝ ╚═════╝╚═╝  ╚═╝ ╚═════╝ `

const GeckoSmall = `🦎 gecko`

// PrintBanner prints the full Gecko startup banner
func PrintBanner() {
	fmt.Println()
	fmt.Printf("%s%s%s%s\n", Bold, GeckoGreen, GeckoAscii, Reset)
	fmt.Printf("  %s%sInfrastructure as Code  •  FOSS-First  •  v0.1.0%s\n", GeckoMuted, Italic, Reset)
	fmt.Printf("  %s──────────────────────────────────────────────────%s\n", GeckoShadow, Reset)
	fmt.Println()
}

// PrintBannerSmall prints a compact banner for subcommands
func PrintBannerSmall(cmd string) {
	fmt.Printf("\n  %s🦎 gecko%s %s%s%s\n", GeckoGreen+Bold, Reset, GeckoTeal, cmd, Reset)
	fmt.Printf("  %s%s%s\n\n", GeckoShadow, strings.Repeat("─", 40), Reset)
}

// Success prints a success message
func Success(msg string) {
	fmt.Printf("  %s✓%s %s%s%s\n", GeckoSuccess+Bold, Reset, BrightWhite, msg, Reset)
}

// Error prints an error message
func Error(msg string) {
	fmt.Printf("  %s✗%s %s%s%s\n", GeckoDanger+Bold, Reset, BrightWhite, msg, Reset)
}

// Warn prints a warning message
func Warn(msg string) {
	fmt.Printf("  %s⚠%s %s%s%s\n", GeckoWarning+Bold, Reset, BrightWhite, msg, Reset)
}

// Info prints an info message
func Info(msg string) {
	fmt.Printf("  %s●%s %s%s%s\n", GeckoInfo+Bold, Reset, White, msg, Reset)
}

// Step prints a numbered step
func Step(n int, total int, msg string) {
	fmt.Printf("  %s[%d/%d]%s %s%s%s\n", GeckoMuted, n, total, Reset, BrightWhite, msg, Reset)
}

// Indent prints an indented detail line
func Indent(msg string) {
	fmt.Printf("     %s%s%s\n", GeckoMuted, msg, Reset)
}

// Header prints a section header
func Header(title string) {
	fmt.Printf("\n  %s%s%s %s%s%s\n", GeckoGreen, Bold, "▸", BrightWhite, title, Reset)
}

// Divider prints a styled divider
func Divider() {
	fmt.Printf("  %s%s%s\n", GeckoShadow, strings.Repeat("─", 52), Reset)
}

// Label prints a key:value pair
func Label(key, value string) {
	fmt.Printf("  %s%-20s%s %s%s%s\n", GeckoMuted, key, Reset, GeckoTeal, value, Reset)
}

// ResourceLine prints a resource change line (plan output)
func ResourceLine(symbol, color, resourceType, name, detail string) {
	fmt.Printf("  %s%s%s %-30s %s%s%s %s%s%s\n",
		color+Bold, symbol, Reset,
		BrightWhite+resourceType+"."+name+Reset,
		GeckoMuted, detail, Reset,
		color, "", Reset,
	)
}

// PlanAdd prints an addition in plan output
func PlanAdd(resourceType, name string) {
	fmt.Printf("  %s+%s %-40s %s(create)%s\n",
		GeckoSuccess+Bold, Reset,
		GeckoSuccess+resourceType+"."+name+Reset,
		GeckoMuted, Reset)
}

// PlanChange prints a change in plan output
func PlanChange(resourceType, name string) {
	fmt.Printf("  %s~%s %-40s %s(update in-place)%s\n",
		GeckoWarning+Bold, Reset,
		GeckoWarning+resourceType+"."+name+Reset,
		GeckoMuted, Reset)
}

// PlanDestroy prints a destruction in plan output
func PlanDestroy(resourceType, name string) {
	fmt.Printf("  %s-%s %-40s %s(destroy)%s\n",
		GeckoDanger+Bold, Reset,
		GeckoDanger+resourceType+"."+name+Reset,
		GeckoMuted, Reset)
}

// PlanNoChange prints a no-change line
func PlanNoChange(resourceType, name string) {
	fmt.Printf("  %s≡%s %-40s %s(no changes)%s\n",
		GeckoMuted+Bold, Reset,
		GeckoMuted+resourceType+"."+name+Reset,
		GeckoMuted, Reset)
}

// PlanSummary prints the plan summary box
func PlanSummary(adds, changes, destroys int) {
	fmt.Println()
	Divider()
	fmt.Printf("  %s%sPlan:%s ", Bold, BrightWhite, Reset)
	if adds > 0 {
		fmt.Printf("%s%s%d to create%s  ", GeckoSuccess, Bold, adds, Reset)
	}
	if changes > 0 {
		fmt.Printf("%s%s%d to change%s  ", GeckoWarning, Bold, changes, Reset)
	}
	if destroys > 0 {
		fmt.Printf("%s%s%d to destroy%s", GeckoDanger, Bold, destroys, Reset)
	}
	if adds == 0 && changes == 0 && destroys == 0 {
		fmt.Printf("%s%sNo changes — infrastructure is up to date%s", GeckoMuted, Italic, Reset)
	}
	fmt.Println()
	Divider()
	fmt.Println()
}

// Spinner is a simple terminal spinner
type Spinner struct {
	frames []string
	msg    string
	done   chan struct{}
}

// NewSpinner creates a new gecko-themed spinner
func NewSpinner(msg string) *Spinner {
	return &Spinner{
		frames: []string{"🦎", "🦎", " 🦎", "  🦎", "   🦎", "    🦎", "   🦎", "  🦎", " 🦎"},
		msg:    msg,
		done:   make(chan struct{}),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() {
	go func() {
		tick := time.NewTicker(120 * time.Millisecond)
		defer tick.Stop()
		dotFrames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
		i := 0
		for {
			select {
			case <-s.done:
				return
			case <-tick.C:
				fmt.Printf("\r  %s%s%s %s%s%s  ",
					GeckoGreen+Bold, dotFrames[i%len(dotFrames)], Reset,
					GeckoMuted+Italic, s.msg, Reset)
				i++
			}
		}
	}()
}

// Stop stops the spinner
func (s *Spinner) Stop(success bool) {
	close(s.done)
	time.Sleep(150 * time.Millisecond)
	if success {
		fmt.Printf("\r  %s✓%s %s%s%s\n", GeckoSuccess+Bold, Reset, BrightWhite, s.msg, Reset)
	} else {
		fmt.Printf("\r  %s✗%s %s%s%s\n", GeckoDanger+Bold, Reset, BrightWhite, s.msg, Reset)
	}
}

// ProgressBar renders a styled progress bar
func ProgressBar(current, total int, label string) {
	width := 30
	pct := float64(current) / float64(total)
	filled := int(math.Round(pct * float64(width)))
	if filled > width {
		filled = width
	}
	bar := strings.Repeat("█", filled) + strings.Repeat("░", width-filled)
	fmt.Printf("\r  %s%s%s %s%3d%%%s  %s",
		GeckoGreen, bar, Reset,
		GeckoTeal+Bold, int(pct*100), Reset,
		GeckoMuted+label+Reset,
	)
	if current == total {
		fmt.Println()
	}
}

// TableHeader prints a styled table header
func TableHeader(cols ...string) {
	fmt.Printf("  %s%s", GeckoShadow, Bold)
	for i, col := range cols {
		if i == 0 {
			fmt.Printf("%-30s", col)
		} else {
			fmt.Printf("  %-20s", col)
		}
	}
	fmt.Printf("%s\n", Reset)
	fmt.Printf("  %s%s%s\n", GeckoShadow, strings.Repeat("─", 72), Reset)
}

// TableRow prints a styled table row
func TableRow(cols ...string) {
	fmt.Printf("  ")
	for i, col := range cols {
		if i == 0 {
			fmt.Printf("%s%-30s%s", BrightWhite, col, Reset)
		} else {
			fmt.Printf("  %s%-20s%s", GeckoMuted, col, Reset)
		}
	}
	fmt.Println()
}

// Confirm asks the user for yes/no confirmation
func Confirm(msg string) bool {
	fmt.Printf("\n  %s%s?%s %s%s%s %s[y/N]%s ", GeckoWarning, Bold, Reset, BrightWhite, msg, Reset, GeckoMuted, Reset)
	var response string
	_, _ = fmt.Scanln(&response)
	response = strings.ToLower(strings.TrimSpace(response))
	return response == "y" || response == "yes"
}

// Fatal prints an error and exits
func Fatal(msg string) {
	Error(msg)
	os.Exit(1)
}

// Tag renders a colored tag/badge
func Tag(label, color string) string {
	return fmt.Sprintf("%s%s %s %s", color+Bold, label, Reset, Reset)
}

// StatusTag renders provider/resource status tags
func StatusTag(status string) string {
	switch strings.ToLower(status) {
	case "running", "active", "healthy", "ok":
		return fmt.Sprintf("%s%s ● %s%s", GeckoSuccess+Bold, "", status, Reset)
	case "pending", "creating", "updating":
		return fmt.Sprintf("%s%s ◌ %s%s", GeckoWarning+Bold, "", status, Reset)
	case "failed", "error", "destroyed":
		return fmt.Sprintf("%s%s ✗ %s%s", GeckoDanger+Bold, "", status, Reset)
	default:
		return fmt.Sprintf("%s%s ─ %s%s", GeckoMuted+Bold, "", status, Reset)
	}
}

// LogLine prints a tail-style log line with timestamp and level
func LogLine(ts, level, source, msg string) {
	var levelColor string
	switch strings.ToUpper(level) {
	case "INFO":
		levelColor = GeckoInfo
	case "WARN":
		levelColor = GeckoWarning
	case "ERROR":
		levelColor = GeckoDanger
	case "DEBUG":
		levelColor = GeckoMuted
	default:
		levelColor = GeckoMuted
	}
	fmt.Printf("  %s%s%s  %s%s%-5s%s  %s%-20s%s  %s\n",
		GeckoShadow, ts, Reset,
		levelColor+Bold, "[", level, "]"+Reset,
		GeckoTeal, source, Reset,
		msg)
}
