package lang

import (
	"fmt"
	"path/filepath"
)

// SnackInstance holds the evaluated result of a single snack import —
// its spawned resources and emitted signal values.
type SnackInstance struct {
	Name      string            // local alias used in the .scute file
	Resources []ResourceResult  // resources this snack contributes
	Signals   map[string]Value  // emitted values accessible as snack_name.signal
}

// EvalSnack resolves a SnackImport: fetches the .snack file, finds the right
// export block, binds caller-supplied params over defaults, evaluates all
// spawn and emit blocks in an isolated scope, and returns the instance.
func (e *Evaluator) EvalSnack(si *SnackImport) (*SnackInstance, error) {
	cacheDir := CacheDir()
	callerDir := filepath.Dir(e.filename)

	// 1. Fetch the snack source
	data, _, err := FetchSnack(si.Source, callerDir, cacheDir)
	if err != nil {
		return nil, fmt.Errorf("snack %q: %w", si.Name, err)
	}

	// 2. Parse the .snack file
	snackFile, parseErrs := ParseSnackFile(string(data), si.Source)
	if len(parseErrs) > 0 {
		return nil, fmt.Errorf("snack %q parse error: %v", si.Name, parseErrs[0])
	}

	// 3. Find the named export (or the only export if there's just one)
	export := findExport(snackFile, si.Name)
	if export == nil && len(snackFile.Exports) == 1 {
		export = snackFile.Exports[0]
	}
	if export == nil {
		available := exportNames(snackFile)
		return nil, fmt.Errorf(
			"snack %q: no export named %q in %s\n  Available exports: %v",
			si.Name, si.Name, si.Source, available,
		)
	}

	// 4. Build a child evaluator scoped to the snack's directory
	child := NewEvaluator(si.Source)
	child.result.Workspace = e.result.Workspace

	// Inherit parent scope so snacks can reference marks from the calling file
	child.scope = newScope(e.scope)

	// 5. Bind params: start with export defaults, overlay caller overrides
	paramMap := make(map[string]*Field)
	for _, f := range si.Params {
		paramMap[f.Key] = f
	}
	for _, pd := range export.Params {
		var val Value = NullVal{}
		if pd.Default != nil {
			// Evaluate default in PARENT scope so it can reference parent marks
			val = e.evalExpr(pd.Default)
		}
		if override, ok := paramMap[pd.Name]; ok {
			// Evaluate override in parent scope too
			val = e.evalExpr(override.Value)
		}
		child.marks[pd.Name] = val
		child.scope.set(pd.Name, val)
	}
	// Also bind any extra params that don't match a declared param (pass-through)
	for k, f := range paramMap {
		if _, declared := child.marks[k]; !declared {
			child.marks[k] = e.evalExpr(f.Value)
			child.scope.set(k, child.marks[k])
		}
	}

	// 6. Evaluate internal camouflage if present
	if export.Camo != nil {
		child.evalCamouflage(export.Camo)
	}

	// 7. Evaluate spawn blocks — these become real resources
	for _, sb := range export.Spawns {
		// Namespace resource names: snack_alias-resource_name to avoid collisions
		originalName := sb.Name
		sb.Name = si.Name + "-" + originalName
		child.evalSpawn(sb)
		sb.Name = originalName // restore for re-use
	}

	// 8. Evaluate emit blocks — these become the snack's signal surface
	signals := make(map[string]Value)
	for _, eb := range export.Emits {
		if eb.Value != nil {
			signals[eb.Name] = child.evalExpr(eb.Value)
		}
	}

	// 9. Collect all resources this snack contributed
	inst := &SnackInstance{
		Name:      si.Name,
		Resources: child.result.Resources,
		Signals:   signals,
	}

	// 10. Register the snack's signals in the parent scope as snack_name.signal
	//     so callers can write: signal "url" \n  value: monitoring.endpoint \n end
	signalMap := make(map[string]Value, len(signals))
	for k, v := range signals {
		signalMap[k] = v
	}
	e.scope.set(si.Name, MapVal{V: signalMap})

	return inst, nil
}

// findExport searches a SnackFile for an export whose name matches the alias.
// Matching is done on the export's declared name, not the caller's alias.
func findExport(sf *SnackFile, alias string) *ExportBlock {
	for _, ex := range sf.Exports {
		if ex.Name == alias {
			return ex
		}
	}
	return nil
}

func exportNames(sf *SnackFile) []string {
	names := make([]string, len(sf.Exports))
	for i, ex := range sf.Exports {
		names[i] = ex.Name
	}
	return names
}
