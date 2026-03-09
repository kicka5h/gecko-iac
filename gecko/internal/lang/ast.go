package lang

import "fmt"

// Node is the base AST interface
type Node interface {
	nodePos() Pos
	nodeString() string
}

// ─── Top-Level ────────────────────────────────────────────────────────────────

// Program is the root node of a parsed .scute file
type Program struct {
	Pos        Pos
	Statements []Node
}

func (p *Program) nodePos() Pos       { return p.Pos }
func (p *Program) nodeString() string { return fmt.Sprintf("Program(%d)", len(p.Statements)) }

// ─── Declarations ─────────────────────────────────────────────────────────────

// TerritoryBlock is the project-level metadata block
//
//	territory "my-cluster"
//	  workspace: "dev"
//	end
type TerritoryBlock struct {
	Pos    Pos
	Name   string
	Fields []*Field
}

func (b *TerritoryBlock) nodePos() Pos       { return b.Pos }
func (b *TerritoryBlock) nodeString() string { return fmt.Sprintf("territory %q", b.Name) }

// HabitatBlock is a provider configuration block
//
//	habitat "k8s"
//	  kubeconfig: "~/.kube/config"
//	end
type HabitatBlock struct {
	Pos    Pos
	Name   string
	Fields []*Field
}

func (b *HabitatBlock) nodePos() Pos       { return b.Pos }
func (b *HabitatBlock) nodeString() string { return fmt.Sprintf("habitat %q", b.Name) }

// SpawnBlock declares a managed resource
//
//	spawn "k8s:namespace" as "monitoring"
//	  name: app
//	  when debug
//	    log_level: "debug"
//	  end
//	end
//
// Protected variant: spawn! (cannot be molted/destroyed)
type SpawnBlock struct {
	Pos       Pos
	TypeStr   string // e.g. "k8s:namespace"
	Name      string // e.g. "monitoring"
	Across    Node   // if non-nil: iteration expression
	Protected bool   // spawn! syntax
	Fields    []*Field
	Whens     []*WhenBlock
	Needs     []Node // explicit @resource dependencies
}

func (b *SpawnBlock) nodePos() Pos       { return b.Pos }
func (b *SpawnBlock) nodeString() string {
	return fmt.Sprintf("spawn %q as %q", b.TypeStr, b.Name)
}

// MarkDecl declares a typed input variable
//
//	mark replicas number: 3
//	mark app_name string: "myapp"
//	mark debug    bool:   false
type MarkDecl struct {
	Pos      Pos
	Name     string
	TypeHint TokenType // TYPE_STRING, TYPE_NUMBER, etc.
	Default  Node
	Description string
}

func (m *MarkDecl) nodePos() Pos       { return m.Pos }
func (m *MarkDecl) nodeString() string { return fmt.Sprintf("mark %s", m.Name) }

// CamouflageBlock declares locally-computed values (like locals in HCL, but named
// after a gecko's ability to adapt its appearance to context)
//
//	camouflage
//	  is_prod: workspace == "prod"
//	  prefix:  "#{app}-#{workspace}"
//	end
type CamouflageBlock struct {
	Pos    Pos
	Fields []*Field
}

func (b *CamouflageBlock) nodePos() Pos       { return b.Pos }
func (b *CamouflageBlock) nodeString() string { return "camouflage" }

// SignalBlock declares a stack output (what the stack broadcasts to callers)
//
//	signal "grafana_url"
//	  value:       "https://grafana.#{base_domain}"
//	  description: "Grafana dashboard endpoint"
//	  sensitive:   false
//	end
type SignalBlock struct {
	Pos         Pos
	Name        string
	Value       Node
	Description string
	Sensitive   bool
}

func (b *SignalBlock) nodePos() Pos       { return b.Pos }
func (b *SignalBlock) nodeString() string { return fmt.Sprintf("signal %q", b.Name) }

// ─── Sub-structures ───────────────────────────────────────────────────────────

// Field is a key: value pair inside any block
type Field struct {
	Pos   Pos
	Key   string
	Value Node
}

func (f *Field) nodePos() Pos       { return f.Pos }
func (f *Field) nodeString() string { return fmt.Sprintf("%s: ...", f.Key) }

// WhenBlock is a conditional inside a spawn block
//
//	when debug
//	  log_level: "debug"
//	else
//	  log_level: "info"
//	end
//
// unless negates the condition
type WhenBlock struct {
	Pos       Pos
	Negate    bool // true if "unless"
	Condition Node
	Then      []*Field
	Else      []*Field
}

func (w *WhenBlock) nodePos() Pos       { return w.Pos }
func (w *WhenBlock) nodeString() string { return "when ..." }

// ─── Expressions ──────────────────────────────────────────────────────────────

// StringLiteral holds a string that may contain #{...} interpolation segments
type StringLiteral struct {
	Pos   Pos
	Parts []StringPart
	Raw   string
}

func (s *StringLiteral) nodePos() Pos       { return s.Pos }
func (s *StringLiteral) nodeString() string { return fmt.Sprintf("%q", s.Raw) }

// StringPart is one segment of a potentially-interpolated string
type StringPart struct {
	IsInterp bool
	Text     string
	Expr     Node
}

// NumberLiteral is an integer or float
type NumberLiteral struct {
	Pos   Pos
	Value float64
	Raw   string
}

func (n *NumberLiteral) nodePos() Pos       { return n.Pos }
func (n *NumberLiteral) nodeString() string { return n.Raw }

// BoolLiteral is true or false
type BoolLiteral struct {
	Pos   Pos
	Value bool
}

func (b *BoolLiteral) nodePos() Pos       { return b.Pos }
func (b *BoolLiteral) nodeString() string { return fmt.Sprintf("%v", b.Value) }

// NullLiteral is null
type NullLiteral struct{ Pos Pos }

func (n *NullLiteral) nodePos() Pos       { return n.Pos }
func (n *NullLiteral) nodeString() string { return "null" }

// SubBlock is an inline nested block (e.g. resources: ... end)
// This is how Scute handles nested objects — no braces, just indented fields + end
type SubBlock struct {
	Pos    Pos
	Fields []*Field
}

func (s *SubBlock) nodePos() Pos       { return s.Pos }
func (s *SubBlock) nodeString() string { return fmt.Sprintf("subblock(%d fields)", len(s.Fields)) }

// ArrayLiteral is [ expr, expr, ... ]
type ArrayLiteral struct {
	Pos      Pos
	Elements []Node
}

func (a *ArrayLiteral) nodePos() Pos       { return a.Pos }
func (a *ArrayLiteral) nodeString() string { return fmt.Sprintf("array(%d)", len(a.Elements)) }

// ResourceRef is @name or @name.field — a cross-resource reference
// The @ sigil makes dependencies visually explicit and scannable
type ResourceRef struct {
	Pos   Pos
	Name  string
	Field string // empty if just @name
}

func (r *ResourceRef) nodePos() Pos       { return r.Pos }
func (r *ResourceRef) nodeString() string {
	if r.Field != "" {
		return fmt.Sprintf("@%s.%s", r.Name, r.Field)
	}
	return "@" + r.Name
}

// Reference is a dotted path to a variable: workspace, camouflage.is_prod, item, etc.
type Reference struct {
	Pos   Pos
	Parts []string
}

func (r *Reference) nodePos() Pos       { return r.Pos }
func (r *Reference) nodeString() string { return joinDots(r.Parts) }

// FunctionCall is func(arg1, arg2)
type FunctionCall struct {
	Pos  Pos
	Name string
	Args []Node
}

func (f *FunctionCall) nodePos() Pos       { return f.Pos }
func (f *FunctionCall) nodeString() string { return fmt.Sprintf("%s(...)", f.Name) }

// BinaryExpr is left op right
type BinaryExpr struct {
	Pos   Pos
	Op    string
	Left  Node
	Right Node
}

func (b *BinaryExpr) nodePos() Pos       { return b.Pos }
func (b *BinaryExpr) nodeString() string {
	return fmt.Sprintf("(%s %s %s)", b.Left.nodeString(), b.Op, b.Right.nodeString())
}

// UnaryExpr is op expr
type UnaryExpr struct {
	Pos  Pos
	Op   string
	Expr Node
}

func (u *UnaryExpr) nodePos() Pos       { return u.Pos }
func (u *UnaryExpr) nodeString() string { return fmt.Sprintf("%s(%s)", u.Op, u.Expr.nodeString()) }

// TernaryExpr is condition ? then_val : else_val
type TernaryExpr struct {
	Pos       Pos
	Condition Node
	Then      Node
	Else      Node
}

func (t *TernaryExpr) nodePos() Pos       { return t.Pos }
func (t *TernaryExpr) nodeString() string { return "ternary" }

// PipeExpr is left | right — null-coalesce: use left unless null, then right
// The pipe character is evocative: "flow to fallback"
type PipeExpr struct {
	Pos   Pos
	Left  Node
	Right Node
}

func (p *PipeExpr) nodePos() Pos       { return p.Pos }
func (p *PipeExpr) nodeString() string { return "pipe" }

// RangeExpr is start..end — produces a list of numbers
type RangeExpr struct {
	Pos   Pos
	Start Node
	End   Node
}

func (r *RangeExpr) nodePos() Pos       { return r.Pos }
func (r *RangeExpr) nodeString() string { return "range" }

func joinDots(parts []string) string {
	result := ""
	for i, p := range parts {
		if i > 0 {
			result += "."
		}
		result += p
	}
	return result
}

// ─── Snack AST Nodes ──────────────────────────────────────────────────────────

// SnackImport is a `snack "name" from "source"` declaration in a .scute file.
// It instantiates a named export from a .snack file, passing parameter overrides.
//
//	snack "monitoring" from "./modules/monitoring.snack"
//	  namespace: "observability"
//	  replicas:  3
//	end
type SnackImport struct {
	Pos    Pos
	Name   string // local alias for this snack instance
	Source string // file path, URL, or registry ref
	Params []*Field
}

func (s *SnackImport) nodePos() Pos       { return s.Pos }
func (s *SnackImport) nodeString() string { return fmt.Sprintf("snack %q from %q", s.Name, s.Source) }

// SnackFile is the root AST node of a .snack file.
// A snack file contains one or more ExportBlock definitions.
type SnackFile struct {
	Pos     Pos
	Exports []*ExportBlock
}

func (f *SnackFile) nodePos() Pos       { return f.Pos }
func (f *SnackFile) nodeString() string { return fmt.Sprintf("snack(%d exports)", len(f.Exports)) }

// ExportBlock defines a reusable module template.
//
//	export "monitoring-stack"
//	  param namespace  string: "monitoring"
//	  param replicas   number: 2
//	  spawn "k8s:namespace" as "ns"
//	    name: namespace
//	  end
//	  emit "endpoint"
//	    value: "http://prometheus.#{namespace}.svc:9090"
//	  end
//	end
type ExportBlock struct {
	Pos    Pos
	Name   string       // export name, matched against snack import
	Params []*ParamDecl // declared parameters (with defaults)
	Spawns []*SpawnBlock
	Camo   *CamouflageBlock // optional internal camouflage
	Emits  []*EmitBlock     // named outputs from this export
}

func (e *ExportBlock) nodePos() Pos       { return e.Pos }
func (e *ExportBlock) nodeString() string { return fmt.Sprintf("export %q", e.Name) }

// ParamDecl declares a parameter within an export block.
//
//	param namespace  string: "monitoring"
//	param replicas   number: 2
type ParamDecl struct {
	Pos      Pos
	Name     string
	TypeHint TokenType
	Default  Node
}

func (p *ParamDecl) nodePos() Pos       { return p.Pos }
func (p *ParamDecl) nodeString() string { return fmt.Sprintf("param %s", p.Name) }

// EmitBlock declares a named output from an export block.
// These become accessible as snack_name.signal_name in the consuming .scute file.
//
//	emit "endpoint"
//	  value: "http://prometheus.svc:9090"
//	end
type EmitBlock struct {
	Pos   Pos
	Name  string
	Value Node
}

func (e *EmitBlock) nodePos() Pos       { return e.Pos }
func (e *EmitBlock) nodeString() string { return fmt.Sprintf("emit %q", e.Name) }

// SnackRef is a reference to a snack's emitted value: snack_name.signal_name
// Distinct from @resource references — no @ sigil, dotted path with snack name first.
type SnackRef struct {
	Pos        Pos
	SnackName  string
	SignalName string
}

func (r *SnackRef) nodePos() Pos       { return r.Pos }
func (r *SnackRef) nodeString() string { return fmt.Sprintf("%s.%s", r.SnackName, r.SignalName) }
