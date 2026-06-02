package ingitdb

import (
	"fmt"
	"maps"
	"math"
	"sync"

	"go.starlark.net/starlark"
	"go.starlark.net/syntax"
)

// maxFormulaSteps caps the number of Starlark execution steps a single formula
// evaluation may take. It is a safety ceiling against pathological expressions
// (e.g. a comprehension over a large range) that could otherwise hang or
// exhaust memory while computing a column on read. Legitimate single-record
// formulas use a few dozen steps; this ceiling is far above any real formula
// yet bounds abuse.
const maxFormulaSteps = 1_000_000

// formulaResultVar is the synthetic global the compiled program assigns the
// formula's value to. The name is deliberately unlikely to collide with a
// real stored field.
const formulaResultVar = "__ingitdb_formula_result__"

// formulaProgramCache memoises compiled Starlark programs keyed by formula
// source, so a formula is parsed and compiled once rather than on every record
// read. Programs are immutable and safe to Init concurrently.
var formulaProgramCache sync.Map // map[string]*starlark.Program

// EvaluateFormula evaluates a computed-column formula as a single Starlark
// expression in a deterministic, side-effect-free sandbox.
//
// Each entry of fields is bound as a top-level variable available to the
// expression. Supported Go input types are string, bool, int, int8, int16,
// int32, int64, uint, uint8, uint16, uint32, uint64, float32, and float64.
// The result is converted back to a Go-native value: string, bool, int64, or
// float64. A nil field value or a None result maps to/from Go nil.
//
// The sandbox exposes no network, filesystem, clock, or randomness, and
// installs no load() loader, so evaluation has zero side effects and is
// deterministic: identical formula and fields always yield identical output.
// Evaluation is bounded by maxFormulaSteps. The compiled program is cached, so
// repeated reads of the same formula do not re-parse it.
func EvaluateFormula(formula string, fields map[string]any) (any, error) {
	prog, err := compileFormula(formula)
	if err != nil {
		return nil, err
	}

	// Start from a clone of Starlark's universe so the standard deterministic
	// builtins (len, min, max, native string methods, ...) remain available,
	// add the curated numeric helpers, then bind the record's fields last so a
	// field shadows a same-named builtin.
	predeclared := maps.Clone(starlark.Universe)
	addFormulaBuiltins(predeclared)
	for name, raw := range fields {
		v, convErr := goToStarlark(raw)
		if convErr != nil {
			return nil, fmt.Errorf("field '%s': %w", name, convErr)
		}
		predeclared[name] = v
	}

	thread := &starlark.Thread{
		Name: "formula",
		// Route print to a no-op so a reachable print() has no side effect.
		Print: func(_ *starlark.Thread, _ string) {},
	}
	thread.SetMaxExecutionSteps(maxFormulaSteps)

	globals, err := prog.Init(thread, predeclared)
	if err != nil {
		return nil, err
	}
	// The compiled program is a single assignment to formulaResultVar, so a
	// successful Init always binds it.
	return starlarkToGo(globals[formulaResultVar])
}

// compileFormula returns the cached compiled program for a formula, compiling
// and memoising it on first use. The formula is wrapped as a single assignment
// so its value is readable from the program's globals after Init. Every free
// identifier is treated as predeclared, making the compiled program independent
// of any particular record's fields and therefore safe to cache by source.
func compileFormula(formula string) (*starlark.Program, error) {
	if cached, ok := formulaProgramCache.Load(formula); ok {
		return cached.(*starlark.Program), nil
	}
	src := formulaResultVar + " = (" + formula + ")\n"
	var opts syntax.FileOptions
	_, prog, err := starlark.SourceProgramOptions(&opts, "formula", src, func(string) bool { return true })
	if err != nil {
		return nil, err
	}
	actual, _ := formulaProgramCache.LoadOrStore(formula, prog)
	return actual.(*starlark.Program), nil
}

// goToStarlark converts a supported Go value into its Starlark equivalent.
func goToStarlark(v any) (starlark.Value, error) {
	switch t := v.(type) {
	case nil:
		return starlark.None, nil
	case bool:
		return starlark.Bool(t), nil
	case string:
		return starlark.String(t), nil
	case int:
		return starlark.MakeInt64(int64(t)), nil
	case int8:
		return starlark.MakeInt64(int64(t)), nil
	case int16:
		return starlark.MakeInt64(int64(t)), nil
	case int32:
		return starlark.MakeInt64(int64(t)), nil
	case int64:
		return starlark.MakeInt64(t), nil
	case uint:
		return starlark.MakeUint64(uint64(t)), nil
	case uint8:
		return starlark.MakeUint64(uint64(t)), nil
	case uint16:
		return starlark.MakeUint64(uint64(t)), nil
	case uint32:
		return starlark.MakeUint64(uint64(t)), nil
	case uint64:
		return starlark.MakeUint64(t), nil
	case float32:
		return starlark.Float(float64(t)), nil
	case float64:
		return starlark.Float(t), nil
	default:
		return nil, fmt.Errorf("unsupported field type %T", v)
	}
}

// starlarkToGo converts a Starlark result value into a Go-native value.
func starlarkToGo(v starlark.Value) (any, error) {
	switch t := v.(type) {
	case starlark.NoneType:
		return nil, nil
	case starlark.Bool:
		return bool(t), nil
	case starlark.String:
		return string(t), nil
	case starlark.Int:
		i, ok := t.Int64()
		if !ok {
			return nil, fmt.Errorf("integer result %s does not fit in int64", t.String())
		}
		return i, nil
	case starlark.Float:
		return float64(t), nil
	default:
		return nil, fmt.Errorf("unsupported result type %s", v.Type())
	}
}

// addFormulaBuiltins installs the curated, deterministic numeric helpers as
// bare top-level names: abs, round, floor, and ceil. Starlark's universe
// already provides len, min, max, and the native string methods; no
// IO-capable or non-deterministic module is exposed.
func addFormulaBuiltins(env starlark.StringDict) {
	env["abs"] = starlark.NewBuiltin("abs", formulaAbs)
	env["round"] = starlark.NewBuiltin("round", formulaRound)
	env["floor"] = starlark.NewBuiltin("floor", formulaFloor)
	env["ceil"] = starlark.NewBuiltin("ceil", formulaCeil)
}

// formulaAbs returns the absolute value, preserving int vs float.
func formulaAbs(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	var x starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &x); err != nil {
		return nil, err
	}
	switch t := x.(type) {
	case starlark.Int:
		if t.Sign() < 0 {
			return zeroInt.Sub(t), nil
		}
		return t, nil
	case starlark.Float:
		return starlark.Float(math.Abs(float64(t))), nil
	default:
		return nil, fmt.Errorf("%s: got %s, want int or float", b.Name(), x.Type())
	}
}

// formulaRound rounds to the nearest integer and returns an int.
func formulaRound(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return floatUnaryToInt(b, args, kwargs, math.Round)
}

// formulaFloor returns the greatest integer <= x as an int.
func formulaFloor(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return floatUnaryToInt(b, args, kwargs, math.Floor)
}

// formulaCeil returns the least integer >= x as an int.
func formulaCeil(_ *starlark.Thread, b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple) (starlark.Value, error) {
	return floatUnaryToInt(b, args, kwargs, math.Ceil)
}

// zeroInt is a reusable zero used to negate integers without allocation churn.
var zeroInt = starlark.MakeInt(0)

// floatUnaryToInt applies fn to an int-or-float argument and returns an int.
func floatUnaryToInt(b *starlark.Builtin, args starlark.Tuple, kwargs []starlark.Tuple, fn func(float64) float64) (starlark.Value, error) {
	var x starlark.Value
	if err := starlark.UnpackPositionalArgs(b.Name(), args, kwargs, 1, &x); err != nil {
		return nil, err
	}
	switch t := x.(type) {
	case starlark.Int:
		return t, nil
	case starlark.Float:
		return starlark.NumberToInt(starlark.Float(fn(float64(t))))
	default:
		return nil, fmt.Errorf("%s: got %s, want int or float", b.Name(), x.Type())
	}
}
