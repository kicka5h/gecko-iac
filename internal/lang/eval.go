package lang

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/gecko-iac/gecko/internal/core"
)

// ─── Values ───────────────────────────────────────────────────────────────────

type Value interface {
	valueType() string
	String() string
}

type (
	StringVal      struct{ V string }
	NumberVal      struct{ V float64 }
	BoolVal        struct{ V bool }
	NullVal        struct{}
	ListVal        struct{ V []Value }
	MapVal         struct{ V map[string]Value }
	ResourceRefVal struct {
		ID      core.ResourceID
		Outputs map[string]Value
	}
)

func (v StringVal) valueType() string      { return "string" }
func (v NumberVal) valueType() string      { return "number" }
func (v BoolVal) valueType() string        { return "bool" }
func (v NullVal) valueType() string        { return "null" }
func (v ListVal) valueType() string        { return "list" }
func (v MapVal) valueType() string         { return "map" }
func (v ResourceRefVal) valueType() string { return "resource" }

func (v StringVal) String() string { return v.V }
func (v NumberVal) String() string { return strconv.FormatFloat(v.V, 'f', -1, 64) }
func (v BoolVal) String() string   { return strconv.FormatBool(v.V) }
func (v NullVal) String() string   { return "null" }
func (v ResourceRefVal) String() string { return string(v.ID) }
func (v ListVal) String() string {
	parts := make([]string, len(v.V))
	for i, el := range v.V {
		parts[i] = el.String()
	}
	return "[" + strings.Join(parts, ", ") + "]"
}
func (v MapVal) String() string {
	var parts []string
	for k, val := range v.V {
		parts = append(parts, k+": "+val.String())
	}
	return "{" + strings.Join(parts, ", ") + "}"
}

// ─── Scope ────────────────────────────────────────────────────────────────────

type Scope struct {
	parent *Scope
	vars   map[string]Value
}

func newScope(parent *Scope) *Scope {
	return &Scope{parent: parent, vars: make(map[string]Value)}
}

func (s *Scope) set(name string, val Value) { s.vars[name] = val }

func (s *Scope) get(name string) (Value, bool) {
	if v, ok := s.vars[name]; ok {
		return v, true
	}
	if s.parent != nil {
		return s.parent.get(name)
	}
	return nil, false
}

// ─── Eval Result ──────────────────────────────────────────────────────────────

// EvalResult is the output of evaluating a .scute project
type EvalResult struct {
	ProjectName   string
	Workspace     string
	Providers     []ProviderResult
	Resources     []ResourceResult
	Outputs       map[string]Value
	Vars          map[string]Value
	BackendType   string                 // from store block, e.g. "local", "s3"
	BackendConfig map[string]interface{} // from store block fields
}

// ProviderResult is an evaluated provider configuration
type ProviderResult struct {
	Name   string
	Config map[string]interface{}
}

// ResourceResult is a fully-evaluated resource declaration
type ResourceResult struct {
	TypeStr   string
	Name      string
	Protected bool
	Args      core.ResourceArgs
	DependsOn []core.ResourceID
}

// ─── Evaluator ────────────────────────────────────────────────────────────────

// Evaluator walks a Scute AST and produces an EvalResult
type Evaluator struct {
	filename    string
	baseDir     string
	scope       *Scope
	errors      []error
	result      *EvalResult
	spawnByName map[string]*ResourceResult // for @ references
	marks       map[string]Value
	camouflage  map[string]Value
}

// NewEvaluator creates an evaluator for a .scute file
func NewEvaluator(filename string) *Evaluator {
	scope := newScope(nil)
	return &Evaluator{
		filename:    filename,
		baseDir:     filepath.Dir(filename),
		scope:       scope,
		result:      &EvalResult{Outputs: make(map[string]Value), Vars: make(map[string]Value)},
		spawnByName: make(map[string]*ResourceResult),
		marks:       make(map[string]Value),
		camouflage:  make(map[string]Value),
	}
}

// Eval evaluates a Program and returns the result
func (e *Evaluator) Eval(prog *Program) (*EvalResult, []error) {
	// Pass 1: collect all mark declarations (so order doesn't matter)
	for _, stmt := range prog.Statements {
		if m, ok := stmt.(*MarkDecl); ok {
			e.evalMark(m)
		}
	}
	// Pass 2: evaluate everything
	for _, stmt := range prog.Statements {
		if _, ok := stmt.(*MarkDecl); ok {
			continue // already done
		}
		e.evalTop(stmt)
	}
	return e.result, e.errors
}

func (e *Evaluator) err(pos Pos, format string, args ...interface{}) {
	e.errors = append(e.errors, &EvalError{Pos: pos, Message: fmt.Sprintf(format, args...)})
}

// ─── Top-Level ────────────────────────────────────────────────────────────────

func (e *Evaluator) evalTop(node Node) {
	switch n := node.(type) {
	case *TerritoryBlock:
		e.evalTerritory(n)
	case *HabitatBlock:
		e.evalHabitat(n)
	case *SpawnBlock:
		e.evalSpawn(n)
	case *CamouflageBlock:
		e.evalCamouflage(n)
	case *SignalBlock:
		e.evalSignal(n)
	case *StoreBlock:
		e.evalStore(n)
	case *SnackImport:
		inst, err := e.EvalSnack(n)
		if err != nil {
			e.errors = append(e.errors, &EvalError{Pos: n.Pos, Message: err.Error()})
			return
		}
		// Merge snack's resources into our result
		e.result.Resources = append(e.result.Resources, inst.Resources...)
		// Register snack's emitted signals as a map in scope: snack_name.signal
		sigMap := make(map[string]Value, len(inst.Signals))
		for k, v := range inst.Signals {
			sigMap[k] = v
		}
		e.scope.set(n.Name, MapVal{V: sigMap})
	}
}

func (e *Evaluator) evalTerritory(b *TerritoryBlock) {
	e.result.ProjectName = b.Name
	for _, f := range b.Fields {
		val := e.evalExpr(f.Value)
		if f.Key == "workspace" {
			e.result.Workspace = val.String()
			e.scope.set("workspace", val)
		}
	}
}

func (e *Evaluator) evalHabitat(b *HabitatBlock) {
	config := make(map[string]interface{})
	for _, f := range b.Fields {
		config[f.Key] = toInterface(e.evalExpr(f.Value))
	}
	e.result.Providers = append(e.result.Providers, ProviderResult{
		Name:   b.Name,
		Config: config,
	})
}

func (e *Evaluator) evalStore(b *StoreBlock) {
	e.result.BackendType = b.Type
	cfg := make(map[string]interface{})
	for _, f := range b.Fields {
		cfg[f.Key] = toInterface(e.evalExpr(f.Value))
	}
	e.result.BackendConfig = cfg
}

func (e *Evaluator) evalMark(m *MarkDecl) {
	var val Value = NullVal{}
	if m.Default != nil {
		val = e.evalExpr(m.Default)
	}
	// Allow override via GECKO_VAR_<n>
	envKey := "GECKO_VAR_" + strings.ToUpper(strings.ReplaceAll(m.Name, "-", "_"))
	if override := os.Getenv(envKey); override != "" {
		val = coerceToType(override, m.TypeHint)
	}
	e.marks[m.Name] = val
	e.result.Vars[m.Name] = val
	e.scope.set(m.Name, val)
	e.scope.set("mark", MapVal{V: e.markMap()})
}

func (e *Evaluator) markMap() map[string]Value {
	m := make(map[string]Value, len(e.marks))
	for k, v := range e.marks {
		m[k] = v
	}
	return m
}

func (e *Evaluator) evalCamouflage(b *CamouflageBlock) {
	for _, f := range b.Fields {
		val := e.evalExpr(f.Value)
		e.camouflage[f.Key] = val
		e.scope.set(f.Key, val) // camouflage values are directly in scope
	}
	e.scope.set("camouflage", MapVal{V: e.camouflageMap()})
}

func (e *Evaluator) camouflageMap() map[string]Value {
	m := make(map[string]Value, len(e.camouflage))
	for k, v := range e.camouflage {
		m[k] = v
	}
	return m
}

func (e *Evaluator) evalSignal(b *SignalBlock) {
	if b.Value != nil {
		e.result.Outputs[b.Name] = e.evalExpr(b.Value)
	}
}

// ─── Spawn Evaluation ─────────────────────────────────────────────────────────

func (e *Evaluator) evalSpawn(b *SpawnBlock) {
	if b.Across != nil {
		e.evalSpawnAcross(b)
		return
	}

	// Evaluate explicit needs declarations
	var deps []core.ResourceID
	for _, needsExpr := range b.Needs {
		dep := e.evalExpr(needsExpr)
		if ref, ok := dep.(ResourceRefVal); ok {
			if !containsID(deps, ref.ID) {
				deps = append(deps, ref.ID)
			}
		}
	}

	inputs := make(core.Inputs)
	e.collectFields(b.Fields, b.Whens, inputs, &deps)

	rr := &ResourceResult{
		TypeStr:   b.TypeStr,
		Name:      b.Name,
		Protected: b.Protected,
		DependsOn: deps,
		Args: core.ResourceArgs{
			Type:      core.ResourceType(b.TypeStr),
			Name:      b.Name,
			Inputs:    inputs,
			DependsOn: deps,
			Protected: b.Protected,
		},
	}

	e.spawnByName[b.Name] = rr
	// Register the spawn in scope so future @name references resolve
	e.scope.set(b.Name, ResourceRefVal{
		ID:      core.ResourceID(fmt.Sprintf("%s::%s", b.TypeStr, b.Name)),
		Outputs: make(map[string]Value),
	})

	e.result.Resources = append(e.result.Resources, *rr)
}

func (e *Evaluator) evalSpawnAcross(b *SpawnBlock) {
	iter := e.evalExpr(b.Across)

	var items []Value
	switch iv := iter.(type) {
	case ListVal:
		items = iv.V
	case MapVal:
		for k, v := range iv.V {
			items = append(items, MapVal{V: map[string]Value{"key": StringVal{k}, "value": v}})
		}
	default:
		e.err(b.Pos, "across requires a list or map, got %s", iter.valueType())
		return
	}

	for i, item := range items {
		child := newScope(e.scope)
		// item is the direct element; item.key/item.value for maps
		child.set("item", item)
		if mv, ok := item.(MapVal); ok {
			if k, ok2 := mv.V["key"]; ok2 {
				child.set("item.key", k)
			}
			if v, ok2 := mv.V["value"]; ok2 {
				child.set("item.value", v)
			}
		}

		savedScope := e.scope
		e.scope = child

		suffix := fmt.Sprintf("-%d", i)
		if sv, ok := item.(StringVal); ok {
			suffix = "-" + sv.V
		} else if mv, ok := item.(MapVal); ok {
			if v, ok2 := mv.V["value"]; ok2 {
				suffix = "-" + v.String()
			}
		}
		instanceName := b.Name + suffix

		var deps []core.ResourceID
		for _, needsExpr := range b.Needs {
			if ref, ok := e.evalExpr(needsExpr).(ResourceRefVal); ok {
				if !containsID(deps, ref.ID) {
					deps = append(deps, ref.ID)
				}
			}
		}
		inputs := make(core.Inputs)
		e.collectFields(b.Fields, b.Whens, inputs, &deps)

		rr := ResourceResult{
			TypeStr:   b.TypeStr,
			Name:      instanceName,
			Protected: b.Protected,
			DependsOn: deps,
			Args: core.ResourceArgs{
				Type:      core.ResourceType(b.TypeStr),
				Name:      instanceName,
				Inputs:    inputs,
				DependsOn: deps,
				Protected: b.Protected,
			},
		}
		e.result.Resources = append(e.result.Resources, rr)
		e.scope = savedScope
	}
}

// collectFields evaluates fields and when blocks into inputs map
func (e *Evaluator) collectFields(fields []*Field, whens []*WhenBlock, inputs core.Inputs, deps *[]core.ResourceID) {
	for _, f := range fields {
		val := e.evalExpr(f.Value)
		inputs[f.Key] = toInterface(val)
		e.gatherDeps(f.Value, deps)
	}
	for _, w := range whens {
		cond := e.evalExpr(w.Condition)
		active := isTruthy(cond)
		if w.Negate {
			active = !active
		}
		chosen := w.Then
		if !active {
			chosen = w.Else
		}
		for _, f := range chosen {
			val := e.evalExpr(f.Value)
			inputs[f.Key] = toInterface(val)
			e.gatherDeps(f.Value, deps)
		}
	}
}

// gatherDeps walks an expression looking for @resource references to auto-dep
func (e *Evaluator) gatherDeps(node Node, deps *[]core.ResourceID) {
	if node == nil {
		return
	}
	switch n := node.(type) {
	case *ResourceRef:
		if rr, ok := e.spawnByName[n.Name]; ok {
			id := core.ResourceID(fmt.Sprintf("%s::%s", rr.TypeStr, rr.Name))
			if !containsID(*deps, id) {
				*deps = append(*deps, id)
			}
		}
	case *BinaryExpr:
		e.gatherDeps(n.Left, deps)
		e.gatherDeps(n.Right, deps)
	case *StringLiteral:
		for _, part := range n.Parts {
			if part.IsInterp {
				e.gatherDeps(part.Expr, deps)
			}
		}
	case *ArrayLiteral:
		for _, el := range n.Elements {
			e.gatherDeps(el, deps)
		}
	case *SubBlock:
		for _, f := range n.Fields {
			e.gatherDeps(f.Value, deps)
		}
	}
}

// ─── Expression Evaluation ────────────────────────────────────────────────────

func (e *Evaluator) evalExpr(node Node) Value {
	if node == nil {
		return NullVal{}
	}
	switch n := node.(type) {
	case *StringLiteral:
		return e.evalString(n)
	case *NumberLiteral:
		return NumberVal{V: n.Value}
	case *BoolLiteral:
		return BoolVal{V: n.Value}
	case *NullLiteral:
		return NullVal{}
	case *ArrayLiteral:
		vals := make([]Value, len(n.Elements))
		for i, el := range n.Elements {
			vals[i] = e.evalExpr(el)
		}
		return ListVal{V: vals}
	case *SubBlock:
		m := make(map[string]Value)
		for _, f := range n.Fields {
			m[f.Key] = e.evalExpr(f.Value)
		}
		return MapVal{V: m}
	case *ResourceRef:
		return e.evalResourceRef(n)
	case *Reference:
		return e.evalRef(n)
	case *FunctionCall:
		return e.evalFn(n)
	case *BinaryExpr:
		return e.evalBinary(n)
	case *UnaryExpr:
		return e.evalUnary(n)
	case *TernaryExpr:
		if isTruthy(e.evalExpr(n.Condition)) {
			return e.evalExpr(n.Then)
		}
		return e.evalExpr(n.Else)
	case *PipeExpr:
		left := e.evalExpr(n.Left)
		if _, isNull := left.(NullVal); isNull {
			return e.evalExpr(n.Right)
		}
		if sv, ok := left.(StringVal); ok && sv.V == "" {
			return e.evalExpr(n.Right)
		}
		return left
	case *RangeExpr:
		start := int(e.evalExpr(n.Start).(NumberVal).V)
		end := int(e.evalExpr(n.End).(NumberVal).V)
		var vals []Value
		for i := start; i <= end; i++ {
			vals = append(vals, NumberVal{V: float64(i)})
		}
		return ListVal{V: vals}
	}
	return NullVal{}
}

func (e *Evaluator) evalString(n *StringLiteral) Value {
	var sb strings.Builder
	for _, p := range n.Parts {
		if p.IsInterp {
			sb.WriteString(e.evalExpr(p.Expr).String())
		} else {
			sb.WriteString(p.Text)
		}
	}
	return StringVal{V: sb.String()}
}

func (e *Evaluator) evalResourceRef(n *ResourceRef) Value {
	// Resolve @name against known spawns
	if rr, ok := e.spawnByName[n.Name]; ok {
		id := core.ResourceID(fmt.Sprintf("%s::%s", rr.TypeStr, rr.Name))
		if n.Field == "" {
			return ResourceRefVal{ID: id, Outputs: make(map[string]Value)}
		}
		// Return a lazy output placeholder — real value resolved post-apply
		return StringVal{V: fmt.Sprintf("${%s.%s}", string(id), n.Field)}
	}
	// Not yet defined — return a forward-reference placeholder
	ref := "@" + n.Name
	if n.Field != "" {
		ref += "." + n.Field
	}
	e.err(n.Pos, "undefined resource reference: %s (resource %q not declared yet)", ref, n.Name)
	return NullVal{}
}

func (e *Evaluator) evalRef(n *Reference) Value {
	if len(n.Parts) == 0 {
		return NullVal{}
	}
	root := n.Parts[0]

	// item — only valid inside across
	if root == "item" {
		if v, ok := e.scope.get("item"); ok {
			return navPath(v, n.Parts[1:])
		}
		e.err(n.Pos, "'item' is only available inside an 'across' block")
		return NullVal{}
	}

	// env.NAME
	if root == "env" && len(n.Parts) > 1 {
		return StringVal{V: os.Getenv(n.Parts[1])}
	}

	// workspace shorthand
	if root == "workspace" {
		return StringVal{V: e.result.Workspace}
	}

	// Camouflage access: camouflage.is_prod or just is_prod (if in scope)
	if root == "camouflage" && len(n.Parts) > 1 {
		if v, ok := e.camouflage[n.Parts[1]]; ok {
			return navPath(v, n.Parts[2:])
		}
		e.err(n.Pos, "undefined camouflage value: %s", n.Parts[1])
		return NullVal{}
	}

	// Direct mark access: replicas, app_name, etc.
	if v, ok := e.scope.get(root); ok {
		return navPath(v, n.Parts[1:])
	}

	e.err(n.Pos, "undefined reference: %s", joinDots(n.Parts))
	return NullVal{}
}

func navPath(v Value, path []string) Value {
	for _, key := range path {
		switch mv := v.(type) {
		case MapVal:
			if sub, ok := mv.V[key]; ok {
				v = sub
			} else {
				return NullVal{}
			}
		case ResourceRefVal:
			if sub, ok := mv.Outputs[key]; ok {
				v = sub
			} else {
				return StringVal{V: fmt.Sprintf("${ref.%s}", key)}
			}
		default:
			return NullVal{}
		}
	}
	return v
}

func (e *Evaluator) evalBinary(n *BinaryExpr) Value {
	l := e.evalExpr(n.Left)
	r := e.evalExpr(n.Right)
	switch n.Op {
	case "+":
		if lv, ok := l.(NumberVal); ok {
			if rv, ok := r.(NumberVal); ok {
				return NumberVal{V: lv.V + rv.V}
			}
		}
		return StringVal{V: l.String() + r.String()}
	case "-":
		if lv, ok := l.(NumberVal); ok {
			if rv, ok := r.(NumberVal); ok {
				return NumberVal{V: lv.V - rv.V}
			}
		}
	case "*":
		if lv, ok := l.(NumberVal); ok {
			if rv, ok := r.(NumberVal); ok {
				return NumberVal{V: lv.V * rv.V}
			}
		}
	case "/":
		if lv, ok := l.(NumberVal); ok {
			if rv, ok := r.(NumberVal); ok {
				if rv.V == 0 {
					e.err(n.Pos, "division by zero")
					return NullVal{}
				}
				return NumberVal{V: lv.V / rv.V}
			}
		}
	case "%":
		if lv, ok := l.(NumberVal); ok {
			if rv, ok := r.(NumberVal); ok {
				return NumberVal{V: math.Mod(lv.V, rv.V)}
			}
		}
	case "==":
		return BoolVal{V: valEq(l, r)}
	case "!=":
		return BoolVal{V: !valEq(l, r)}
	case "<":
		return BoolVal{V: cmpNum(l, r) < 0}
	case ">":
		return BoolVal{V: cmpNum(l, r) > 0}
	case "<=":
		return BoolVal{V: cmpNum(l, r) <= 0}
	case ">=":
		return BoolVal{V: cmpNum(l, r) >= 0}
	case "&&":
		return BoolVal{V: isTruthy(l) && isTruthy(r)}
	case "||":
		return BoolVal{V: isTruthy(l) || isTruthy(r)}
	}
	return NullVal{}
}

func (e *Evaluator) evalUnary(n *UnaryExpr) Value {
	v := e.evalExpr(n.Expr)
	switch n.Op {
	case "!", "not":
		return BoolVal{V: !isTruthy(v)}
	case "-":
		if nv, ok := v.(NumberVal); ok {
			return NumberVal{V: -nv.V}
		}
	}
	return NullVal{}
}

// ─── Built-in Functions ───────────────────────────────────────────────────────

func (e *Evaluator) evalFn(n *FunctionCall) Value {
	arg := func(i int) Value {
		if i < len(n.Args) {
			return e.evalExpr(n.Args[i])
		}
		return NullVal{}
	}
	argStr := func(i int) string { return arg(i).String() }

	switch n.Name {
	// ── Environment & Secrets ──────────────────────────────────────────────
	case "env":
		return StringVal{V: os.Getenv(argStr(0))}

	case "secret":
		// Resolved by secrets backend at apply-time; return a typed sentinel
		return StringVal{V: fmt.Sprintf("${secret:%s}", argStr(0))}

	case "file":
		path := argStr(0)
		if !filepath.IsAbs(path) {
			path = filepath.Join(e.baseDir, path)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			e.err(n.Pos, "file(): cannot read %q: %v", path, err)
			return NullVal{}
		}
		return StringVal{V: string(data)}

	// ── Type Conversions ───────────────────────────────────────────────────
	case "str", "tostring":
		return StringVal{V: argStr(0)}
	case "num", "tonumber":
		f, err := strconv.ParseFloat(argStr(0), 64)
		if err != nil {
			e.err(n.Pos, "num(): %q is not a number", argStr(0))
			return NullVal{}
		}
		return NumberVal{V: f}
	case "flag", "tobool":
		v := arg(0)
		return BoolVal{V: isTruthy(v)}

	// ── String Functions ───────────────────────────────────────────────────
	case "upper":
		return StringVal{V: strings.ToUpper(argStr(0))}
	case "lower":
		return StringVal{V: strings.ToLower(argStr(0))}
	case "trim":
		return StringVal{V: strings.TrimSpace(argStr(0))}
	case "replace":
		return StringVal{V: strings.ReplaceAll(argStr(0), argStr(1), argStr(2))}
	case "split":
		parts := strings.Split(argStr(1), argStr(0))
		vals := make([]Value, len(parts))
		for i, p := range parts {
			vals[i] = StringVal{V: p}
		}
		return ListVal{V: vals}
	case "join":
		if lv, ok := arg(1).(ListVal); ok {
			parts := make([]string, len(lv.V))
			for i, v := range lv.V {
				parts[i] = v.String()
			}
			return StringVal{V: strings.Join(parts, argStr(0))}
		}
		return NullVal{}
	case "contains":
		s := argStr(0)
		sub := argStr(1)
		return BoolVal{V: strings.Contains(s, sub)}
	case "starts_with":
		return BoolVal{V: strings.HasPrefix(argStr(0), argStr(1))}
	case "ends_with":
		return BoolVal{V: strings.HasSuffix(argStr(0), argStr(1))}
	case "fmt", "format":
		iargs := make([]interface{}, len(n.Args)-1)
		for i := range iargs {
			iargs[i] = argStr(i + 1)
		}
		return StringVal{V: fmt.Sprintf(argStr(0), iargs...)}

	// ── List Functions ─────────────────────────────────────────────────────
	case "len", "length":
		switch v := arg(0).(type) {
		case StringVal:
			return NumberVal{V: float64(len(v.V))}
		case ListVal:
			return NumberVal{V: float64(len(v.V))}
		case MapVal:
			return NumberVal{V: float64(len(v.V))}
		}
		return NumberVal{}
	case "range":
		start := int(arg(0).(NumberVal).V)
		end := int(arg(1).(NumberVal).V)
		var vals []Value
		for i := start; i < end; i++ {
			vals = append(vals, NumberVal{V: float64(i)})
		}
		return ListVal{V: vals}
	case "concat":
		var result []Value
		for _, a := range n.Args {
			if lv, ok := e.evalExpr(a).(ListVal); ok {
				result = append(result, lv.V...)
			}
		}
		return ListVal{V: result}
	case "flatten":
		var result []Value
		flattenInto(arg(0), &result)
		return ListVal{V: result}
	case "unique":
		seen := make(map[string]bool)
		var result []Value
		if lv, ok := arg(0).(ListVal); ok {
			for _, v := range lv.V {
				k := v.String()
				if !seen[k] {
					seen[k] = true
					result = append(result, v)
				}
			}
		}
		return ListVal{V: result}
	case "first":
		if lv, ok := arg(0).(ListVal); ok && len(lv.V) > 0 {
			return lv.V[0]
		}
		return NullVal{}
	case "last":
		if lv, ok := arg(0).(ListVal); ok && len(lv.V) > 0 {
			return lv.V[len(lv.V)-1]
		}
		return NullVal{}
	case "in_list":
		target := arg(0)
		if lv, ok := arg(1).(ListVal); ok {
			for _, v := range lv.V {
				if valEq(v, target) {
					return BoolVal{V: true}
				}
			}
		}
		return BoolVal{V: false}

	// ── Map Functions ──────────────────────────────────────────────────────
	case "keys":
		if mv, ok := arg(0).(MapVal); ok {
			var keys []Value
			for k := range mv.V {
				keys = append(keys, StringVal{V: k})
			}
			return ListVal{V: keys}
		}
		return NullVal{}
	case "values":
		if mv, ok := arg(0).(MapVal); ok {
			var vals []Value
			for _, v := range mv.V {
				vals = append(vals, v)
			}
			return ListVal{V: vals}
		}
		return NullVal{}
	case "merge":
		result := make(map[string]Value)
		for _, a := range n.Args {
			if mv, ok := e.evalExpr(a).(MapVal); ok {
				for k, v := range mv.V {
					result[k] = v
				}
			}
		}
		return MapVal{V: result}
	case "pick":
		// pick(map, key1, key2, ...) → new map with only those keys
		if mv, ok := arg(0).(MapVal); ok {
			result := make(map[string]Value)
			for _, a := range n.Args[1:] {
				k := e.evalExpr(a).String()
				if v, ok := mv.V[k]; ok {
					result[k] = v
				}
			}
			return MapVal{V: result}
		}
		return NullVal{}

	// ── Encoding ───────────────────────────────────────────────────────────
	case "json":
		return StringVal{V: jsonVal(arg(0))}
	case "base64":
		import_b64 := "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"
		_ = import_b64
		return StringVal{V: simpleB64(argStr(0))}
	case "yaml_label":
		// Formats a map as k: v YAML labels for k8s
		if mv, ok := arg(0).(MapVal); ok {
			var lines []string
			for k, v := range mv.V {
				lines = append(lines, fmt.Sprintf("%s: %s", k, v.String()))
			}
			return StringVal{V: strings.Join(lines, "\n")}
		}
		return StringVal{V: ""}

	// ── Math ───────────────────────────────────────────────────────────────
	case "max":
		if len(n.Args) == 0 {
			return NullVal{}
		}
		best := e.evalExpr(n.Args[0])
		for _, a := range n.Args[1:] {
			v := e.evalExpr(a)
			if cmpNum(v, best) > 0 {
				best = v
			}
		}
		return best
	case "min":
		if len(n.Args) == 0 {
			return NullVal{}
		}
		best := e.evalExpr(n.Args[0])
		for _, a := range n.Args[1:] {
			v := e.evalExpr(a)
			if cmpNum(v, best) < 0 {
				best = v
			}
		}
		return best
	case "abs":
		if nv, ok := arg(0).(NumberVal); ok {
			return NumberVal{V: math.Abs(nv.V)}
		}
		return NullVal{}
	case "ceil":
		if nv, ok := arg(0).(NumberVal); ok {
			return NumberVal{V: math.Ceil(nv.V)}
		}
		return NullVal{}
	case "floor":
		if nv, ok := arg(0).(NumberVal); ok {
			return NumberVal{V: math.Floor(nv.V)}
		}
		return NullVal{}

	// ── Nullability ────────────────────────────────────────────────────────
	case "coalesce":
		for _, a := range n.Args {
			v := e.evalExpr(a)
			if _, isNull := v.(NullVal); !isNull {
				if sv, ok := v.(StringVal); ok && sv.V == "" {
					continue
				}
				return v
			}
		}
		return NullVal{}
	case "defined":
		v := arg(0)
		_, isNull := v.(NullVal)
		return BoolVal{V: !isNull}
	case "default":
		v := arg(0)
		if _, isNull := v.(NullVal); isNull {
			return arg(1)
		}
		if sv, ok := v.(StringVal); ok && sv.V == "" {
			return arg(1)
		}
		return v
	}

	e.err(n.Pos, "unknown function: %s() — check the Scute standard library", n.Name)
	return NullVal{}
}

// ─── Helpers ──────────────────────────────────────────────────────────────────

func isTruthy(v Value) bool {
	switch vv := v.(type) {
	case BoolVal:
		return vv.V
	case NullVal:
		return false
	case NumberVal:
		return vv.V != 0
	case StringVal:
		return vv.V != ""
	case ListVal:
		return len(vv.V) > 0
	case MapVal:
		return len(vv.V) > 0
	}
	return false
}

func valEq(a, b Value) bool {
	switch av := a.(type) {
	case StringVal:
		bv, ok := b.(StringVal)
		return ok && av.V == bv.V
	case NumberVal:
		bv, ok := b.(NumberVal)
		return ok && av.V == bv.V
	case BoolVal:
		bv, ok := b.(BoolVal)
		return ok && av.V == bv.V
	case NullVal:
		_, ok := b.(NullVal)
		return ok
	}
	return false
}

func cmpNum(a, b Value) int {
	an, aok := a.(NumberVal)
	bn, bok := b.(NumberVal)
	if aok && bok {
		if an.V < bn.V {
			return -1
		}
		if an.V > bn.V {
			return 1
		}
		return 0
	}
	return strings.Compare(a.String(), b.String())
}

func toInterface(v Value) interface{} {
	switch vv := v.(type) {
	case StringVal:
		return vv.V
	case NumberVal:
		if vv.V == math.Trunc(vv.V) {
			return int(vv.V)
		}
		return vv.V
	case BoolVal:
		return vv.V
	case NullVal:
		return nil
	case ListVal:
		r := make([]interface{}, len(vv.V))
		for i, el := range vv.V {
			r[i] = toInterface(el)
		}
		return r
	case MapVal:
		r := make(map[string]interface{}, len(vv.V))
		for k, val := range vv.V {
			r[k] = toInterface(val)
		}
		return r
	case ResourceRefVal:
		return string(vv.ID)
	}
	return nil
}

func coerceToType(s string, hint TokenType) Value {
	switch hint {
	case TYPE_NUMBER:
		if f, err := strconv.ParseFloat(s, 64); err == nil {
			return NumberVal{V: f}
		}
	case TYPE_BOOL:
		if b, err := strconv.ParseBool(s); err == nil {
			return BoolVal{V: b}
		}
	}
	return StringVal{V: s}
}

func containsID(ids []core.ResourceID, id core.ResourceID) bool {
	for _, existing := range ids {
		if existing == id {
			return true
		}
	}
	return false
}

func jsonVal(v Value) string {
	switch vv := v.(type) {
	case StringVal:
		return strconv.Quote(vv.V)
	case NumberVal:
		return vv.String()
	case BoolVal:
		return strconv.FormatBool(vv.V)
	case NullVal:
		return "null"
	case ListVal:
		parts := make([]string, len(vv.V))
		for i, el := range vv.V {
			parts[i] = jsonVal(el)
		}
		return "[" + strings.Join(parts, ",") + "]"
	case MapVal:
		var parts []string
		for k, val := range vv.V {
			parts = append(parts, strconv.Quote(k)+":"+jsonVal(val))
		}
		return "{" + strings.Join(parts, ",") + "}"
	}
	return "null"
}

func flattenInto(v Value, out *[]Value) {
	if lv, ok := v.(ListVal); ok {
		for _, el := range lv.V {
			flattenInto(el, out)
		}
	} else {
		*out = append(*out, v)
	}
}

var b64chars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/"

func simpleB64(s string) string {
	data := []byte(s)
	var sb strings.Builder
	for i := 0; i < len(data); i += 3 {
		b0 := data[i]
		var b1, b2 byte
		if i+1 < len(data) {
			b1 = data[i+1]
		}
		if i+2 < len(data) {
			b2 = data[i+2]
		}
		sb.WriteByte(b64chars[b0>>2])
		sb.WriteByte(b64chars[((b0&3)<<4)|(b1>>4)])
		if i+1 < len(data) {
			sb.WriteByte(b64chars[((b1&15)<<2)|(b2>>6)])
		} else {
			sb.WriteByte('=')
		}
		if i+2 < len(data) {
			sb.WriteByte(b64chars[b2&63])
		} else {
			sb.WriteByte('=')
		}
	}
	return sb.String()
}
