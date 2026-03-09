package lang

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gecko-iac/gecko/internal/core"
)

// LoadOptions configures how a Scute project is loaded
type LoadOptions struct {
	ProjectDir   string
	Workspace    string
	VarOverrides map[string]string // --var name=value from CLI
}

// LoadedStack holds the result of evaluating a Scute project
type LoadedStack struct {
	Stack         *core.Stack
	ProviderHints []ProviderResult
	Outputs       map[string]Value
	Errors        []error
}

// LoadProject finds, parses, and evaluates all .scute files for a workspace,
// returning a populated core.Stack ready for planning and applying.
func LoadProject(opts LoadOptions) (*LoadedStack, error) {
	dir := opts.ProjectDir
	if dir == "" {
		var err error
		dir, err = FindProjectRoot()
		if err != nil {
			return nil, err
		}
	}

	workspace := opts.Workspace
	if workspace == "" {
		workspace = "dev"
	}

	files, err := FindScuteFiles(dir, workspace)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return nil, fmt.Errorf(
			"no .scute files found in %s (workspace: %s)\n"+
				"  Try: gecko hatch <name> --providers k8s",
			dir, workspace,
		)
	}

	merged := &EvalResult{
		Outputs: make(map[string]Value),
		Vars:    make(map[string]Value),
	}
	var allErrors []error

	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			allErrors = append(allErrors, fmt.Errorf("cannot read %s: %w", f, err))
			continue
		}

		prog, parseErrs := ParseFile(string(src), f)
		allErrors = append(allErrors, parseErrs...)
		if len(parseErrs) > 0 {
			continue
		}

		eval := NewEvaluator(f)
		for k, v := range opts.VarOverrides {
			eval.marks[k] = StringVal{V: v}
			eval.scope.set(k, StringVal{V: v})
		}

		result, evalErrs := eval.Eval(prog)
		allErrors = append(allErrors, evalErrs...)

		if result.ProjectName != "" {
			merged.ProjectName = result.ProjectName
		}
		if result.Workspace != "" {
			merged.Workspace = result.Workspace
		}
		merged.Providers = append(merged.Providers, result.Providers...)
		merged.Resources = append(merged.Resources, result.Resources...)
		for k, v := range result.Outputs {
			merged.Outputs[k] = v
		}
		for k, v := range result.Vars {
			merged.Vars[k] = v
		}
	}

	if countHardErrors(allErrors) > 0 {
		return nil, combineErrors(allErrors)
	}

	if opts.Workspace != "" {
		merged.Workspace = opts.Workspace
	}
	if merged.Workspace == "" {
		merged.Workspace = "dev"
	}
	if merged.ProjectName == "" {
		merged.ProjectName = filepath.Base(dir)
	}

	stack := core.NewStack(merged.ProjectName, "main", merged.Workspace)
	for _, rr := range merged.Resources {
		stack.Resource(rr.Args)
	}

	return &LoadedStack{
		Stack:         stack,
		ProviderHints: merged.Providers,
		Outputs:       merged.Outputs,
	}, nil
}

// FindProjectRoot walks up from cwd looking for gecko.json or *.scute files
func FindProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "gecko.json")); err == nil {
			return dir, nil
		}
		entries, _ := os.ReadDir(dir)
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".scute") {
				return dir, nil
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "", fmt.Errorf("could not find a Gecko project root (no gecko.json or .scute files)")
}

// FindScuteFiles returns the ordered list of .scute files to evaluate
func FindScuteFiles(projectDir, workspace string) ([]string, error) {
	var files []string

	// Priority 1: stacks/<workspace>/*.scute
	stackDir := filepath.Join(projectDir, "stacks", workspace)
	if entries, err := os.ReadDir(stackDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".scute") {
				files = append(files, filepath.Join(stackDir, e.Name()))
			}
		}
	}

	// Priority 2: project root *.scute
	if len(files) == 0 {
		entries, err := os.ReadDir(projectDir)
		if err != nil {
			return nil, err
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".scute") {
				files = append(files, filepath.Join(projectDir, e.Name()))
			}
		}
	}

	return files, nil
}

// ─── Formatter ────────────────────────────────────────────────────────────────

// Format pretty-prints a .scute source file in canonical Scute style.
// Called by `gecko fmt`.
func Format(src, filename string) (string, []error) {
	prog, errs := ParseFile(src, filename)
	if len(errs) > 0 {
		return src, errs
	}
	f := &formatter{}
	f.writeProgram(prog)
	return f.sb.String(), nil
}

type formatter struct {
	sb     strings.Builder
	depth  int
}

func (f *formatter) pad() string  { return strings.Repeat("  ", f.depth) }
func (f *formatter) line(s string) { f.sb.WriteString(f.pad() + s + "\n") }
func (f *formatter) blank()        { f.sb.WriteString("\n") }

func (f *formatter) writeProgram(prog *Program) {
	for i, stmt := range prog.Statements {
		if i > 0 {
			f.blank()
		}
		f.writeNode(stmt)
	}
}

func (f *formatter) writeNode(node Node) {
	switch n := node.(type) {

	case *TerritoryBlock:
		if n.Name != "" {
			f.line(fmt.Sprintf("territory %q", n.Name))
		} else {
			f.line("territory")
		}
		f.depth++
		f.writeFields(n.Fields)
		f.depth--
		f.line("end")

	case *HabitatBlock:
		f.line(fmt.Sprintf("habitat %q", n.Name))
		f.depth++
		f.writeFields(n.Fields)
		f.depth--
		f.line("end")

	case *MarkDecl:
		typeStr := typeHintName(n.TypeHint)
		if n.Default != nil {
			f.line(fmt.Sprintf("mark %-16s %-8s: %s", n.Name, typeStr, f.fmtExpr(n.Default)))
		} else {
			f.line(fmt.Sprintf("mark %-16s %s", n.Name, typeStr))
		}

	case *CamouflageBlock:
		f.line("camouflage")
		f.depth++
		f.writeFieldsAligned(n.Fields)
		f.depth--
		f.line("end")

	case *SpawnBlock:
		keyword := "spawn"
		if n.Protected {
			keyword = "spawn!"
		}
		header := fmt.Sprintf("%s %q as %q", keyword, n.TypeStr, n.Name)
		if n.Across != nil {
			header += " across " + f.fmtExpr(n.Across)
		}
		f.line(header)
		f.depth++
		for _, needsExpr := range n.Needs {
			f.line("needs: " + f.fmtExpr(needsExpr))
		}
		f.writeFieldsAligned(n.Fields)
		for _, w := range n.Whens {
			f.writeWhen(w)
		}
		f.depth--
		f.line("end")

	case *SignalBlock:
		f.line(fmt.Sprintf("signal %q", n.Name))
		f.depth++
		if n.Value != nil {
			f.line("value:       " + f.fmtExpr(n.Value))
		}
		if n.Description != "" {
			f.line(fmt.Sprintf("description: %q", n.Description))
		}
		if n.Sensitive {
			f.line("sensitive:   true")
		}
		f.depth--
		f.line("end")
	}
}

func (f *formatter) writeFields(fields []*Field) {
	for _, field := range fields {
		f.writeField(field)
	}
}

// writeFieldsAligned aligns the ':' across all sibling fields
func (f *formatter) writeFieldsAligned(fields []*Field) {
	maxLen := 0
	for _, field := range fields {
		if len(field.Key) > maxLen {
			maxLen = len(field.Key)
		}
	}
	for _, field := range fields {
		val := f.fmtExpr(field.Value)
		if sub, ok := field.Value.(*SubBlock); ok {
			f.line(field.Key + ":")
			f.depth++
			f.writeFieldsAligned(sub.Fields)
			f.depth--
			f.line("end")
		} else {
			padding := strings.Repeat(" ", maxLen-len(field.Key))
			f.line(fmt.Sprintf("%s%s: %s", field.Key, padding, val))
		}
	}
}

func (f *formatter) writeField(field *Field) {
	if sub, ok := field.Value.(*SubBlock); ok {
		f.line(field.Key + ":")
		f.depth++
		f.writeFields(sub.Fields)
		f.depth--
		f.line("end")
	} else {
		f.line(fmt.Sprintf("%s: %s", field.Key, f.fmtExpr(field.Value)))
	}
}

func (f *formatter) writeWhen(w *WhenBlock) {
	keyword := "when"
	if w.Negate {
		keyword = "unless"
	}
	f.line(fmt.Sprintf("%s %s", keyword, f.fmtExpr(w.Condition)))
	f.depth++
	f.writeFieldsAligned(w.Then)
	f.depth--
	if len(w.Else) > 0 {
		f.line("else")
		f.depth++
		f.writeFieldsAligned(w.Else)
		f.depth--
	}
	f.line("end")
}

func (f *formatter) fmtExpr(node Node) string {
	if node == nil {
		return "null"
	}
	switch n := node.(type) {
	case *StringLiteral:
		return fmtStringLit(n)
	case *NumberLiteral:
		return n.Raw
	case *BoolLiteral:
		if n.Value {
			return "true"
		}
		return "false"
	case *NullLiteral:
		return "null"
	case *ResourceRef:
		if n.Field != "" {
			return fmt.Sprintf("@%s.%s", n.Name, n.Field)
		}
		return "@" + n.Name
	case *Reference:
		return joinDots(n.Parts)
	case *FunctionCall:
		args := make([]string, len(n.Args))
		for i, a := range n.Args {
			args[i] = f.fmtExpr(a)
		}
		return fmt.Sprintf("%s(%s)", n.Name, strings.Join(args, ", "))
	case *BinaryExpr:
		return fmt.Sprintf("%s %s %s", f.fmtExpr(n.Left), n.Op, f.fmtExpr(n.Right))
	case *UnaryExpr:
		return n.Op + f.fmtExpr(n.Expr)
	case *TernaryExpr:
		return fmt.Sprintf("%s ? %s : %s",
			f.fmtExpr(n.Condition), f.fmtExpr(n.Then), f.fmtExpr(n.Else))
	case *PipeExpr:
		return fmt.Sprintf("%s | %s", f.fmtExpr(n.Left), f.fmtExpr(n.Right))
	case *RangeExpr:
		return fmt.Sprintf("%s..%s", f.fmtExpr(n.Start), f.fmtExpr(n.End))
	case *ArrayLiteral:
		if len(n.Elements) == 0 {
			return "[]"
		}
		parts := make([]string, len(n.Elements))
		for i, el := range n.Elements {
			parts[i] = f.fmtExpr(el)
		}
		if len(parts) <= 5 {
			return "[" + strings.Join(parts, ", ") + "]"
		}
		var sb strings.Builder
		sb.WriteString("[\n")
		for _, el := range n.Elements {
			sb.WriteString(f.pad() + "  " + f.fmtExpr(el) + ",\n")
		}
		sb.WriteString(f.pad() + "]")
		return sb.String()
	case *SubBlock:
		if len(n.Fields) == 0 {
			return "{}"
		}
		// Inline sub-blocks aren't formatted inline — they get their own end-block
		return "<sub-block>"
	}
	return "null"
}

func fmtStringLit(n *StringLiteral) string {
	var sb strings.Builder
	sb.WriteRune('"')
	for _, part := range n.Parts {
		if part.IsInterp {
			sb.WriteString("#{...}")
		} else {
			sb.WriteString(strings.ReplaceAll(part.Text, `"`, `\"`))
		}
	}
	sb.WriteRune('"')
	return sb.String()
}

func typeHintName(tt TokenType) string {
	switch tt {
	case TYPE_STRING:
		return "string"
	case TYPE_NUMBER:
		return "number"
	case TYPE_BOOL:
		return "bool"
	case TYPE_LIST:
		return "list"
	case TYPE_MAP:
		return "map"
	case TYPE_ANY:
		return "any"
	}
	return "any"
}

// ─── Checker ──────────────────────────────────────────────────────────────────

// CheckResult is the output of `gecko check`
type CheckResult struct {
	Errors   []error
	Warnings []string
	Stats    CheckStats
}

// CheckStats counts declarations in the project
type CheckStats struct {
	Files     int
	Spawns    int
	Habitats  int
	Marks     int
	Signals   int
}

// Check parses and validates all .scute files without running side effects.
func Check(projectDir, workspace string) *CheckResult {
	result := &CheckResult{}
	files, err := FindScuteFiles(projectDir, workspace)
	if err != nil {
		result.Errors = append(result.Errors, err)
		return result
	}
	result.Stats.Files = len(files)
	for _, f := range files {
		src, err := os.ReadFile(f)
		if err != nil {
			result.Errors = append(result.Errors, err)
			continue
		}
		prog, errs := ParseFile(string(src), f)
		result.Errors = append(result.Errors, errs...)
		for _, stmt := range prog.Statements {
			switch stmt.(type) {
			case *SpawnBlock:
				result.Stats.Spawns++
			case *HabitatBlock:
				result.Stats.Habitats++
			case *MarkDecl:
				result.Stats.Marks++
			case *SignalBlock:
				result.Stats.Signals++
			}
		}
	}
	return result
}

// ─── Error Helpers ────────────────────────────────────────────────────────────

func countHardErrors(errs []error) int {
	count := 0
	for _, e := range errs {
		if e != nil {
			count++
		}
	}
	return count
}

func combineErrors(errs []error) error {
	var msgs []string
	for _, e := range errs {
		if e != nil {
			msgs = append(msgs, "  • "+e.Error())
		}
	}
	return fmt.Errorf("%d error(s) in Scute files:\n%s", len(msgs), strings.Join(msgs, "\n"))
}
