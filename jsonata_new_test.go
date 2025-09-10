package jsonata

import (
	"fmt"
	"sync"
	"testing"
)

func TestExpressionAndEval_Simple(t *testing.T) {
	comp, err := NewCompiler(nil, nil)
	if err != nil {
		t.Fatalf("NewCompiler failed: %v", err)
	}
	expr, err := comp.Compile("1+2")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	out, err := expr.Eval(nil, nil)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}

	got, ok := out.(float64)
	if !ok {
		t.Fatalf("expected float64, got %T (%v)", out, out)
	}
	if got != 3 {
		t.Fatalf("expected 3, got %v", got)
	}
}

func TestExpression_WithExtsAndEval(t *testing.T) {
	comp, err := NewCompiler(nil, map[string]Extension{
		"twice": {Func: func(x float64) float64 { return x * 2 }},
	})
	if err != nil {
		t.Fatalf("NewCompiler failed: %v", err)
	}
	expr, err := comp.Compile("$twice(21)")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	out, err := expr.Eval(nil, nil)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(float64) != 42 {
		t.Fatalf("expected 42, got %v", out)
	}
}

func TestEvaluator_ConcurrentEval(t *testing.T) {
	// Use a contextable builtin via function application to exercise per-call context.
	comp, err := NewCompiler(nil, nil)
	if err != nil {
		t.Fatalf("NewCompiler failed: %v", err)
	}
	expr, err := comp.Compile("'HelloWorld' ~> $substring(5, 5)")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}

	const goroutines = 50
	const iterations = 20

	var wg sync.WaitGroup
	errs := make(chan error, goroutines*iterations)
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			for i := 0; i < iterations; i++ {
				out, err := expr.Eval(nil, nil)
				if err != nil {
					errs <- err
					return
				}
				// substring(5,5) on "HelloWorld" => "World"
				if s, ok := out.(string); !ok || s != "World" {
					errs <- fmt.Errorf("expected World, got %T (%v)", out, out)
					return
				}
			}
		}()
	}

	wg.Wait()

	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent eval error: %v", err)
		}
	}
}

// --- New API compatibility tests mirroring legacy patterns ---

func evalNew(t *testing.T, expression string, input interface{}, vars map[string]interface{}, exts map[string]Extension) (interface{}, error) {
	comp, err := NewCompiler(vars, exts)
	if err != nil {
		t.Fatalf("NewCompiler failed: %v", err)
	}
	expr, err := comp.Compile(expression)
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	return expr.Eval(input, nil)
}

func TestNewAPI_PathsAndLiterals(t *testing.T) {
	data := map[string]interface{}{
		"foo": map[string]interface{}{
			"bar": 42,
			"blah": []interface{}{
				map[string]interface{}{"baz": map[string]interface{}{"fud": "hello"}},
				map[string]interface{}{"baz": map[string]interface{}{"fud": "world"}},
			},
		},
		"bar": 98,
	}

	out, err := evalNew(t, "foo.bar + bar", data, nil, nil)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(float64) != 140 {
		t.Fatalf("expected 140, got %v", out)
	}

	out, err = evalNew(t, "foo.blah[0].baz.fud", data, nil, nil)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(string) != "hello" {
		t.Fatalf("expected hello, got %v", out)
	}
}

func TestNewAPI_VarsAndExts(t *testing.T) {
	vars := map[string]interface{}{
		"greet": "Hello",
	}
	exts := map[string]Extension{
		"twice": {Func: func(x float64) float64 { return x * 2 }},
	}

	out, err := evalNew(t, "$greet & ' ' & $.name", map[string]interface{}{"name": "Ada"}, vars, nil)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(string) != "Hello Ada" {
		t.Fatalf("expected Hello Ada, got %v", out)
	}

	out, err = evalNew(t, "$twice(21)", nil, nil, exts)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(float64) != 42 {
		t.Fatalf("expected 42, got %v", out)
	}
}

func TestNewAPI_ExtrasOverrideBase(t *testing.T) {
	comp, err := NewCompiler(map[string]interface{}{"greet": "Hello"}, nil)
	if err != nil {
		t.Fatalf("NewCompiler failed: %v", err)
	}
	expr, err := comp.Compile("$greet")
	if err != nil {
		t.Fatalf("Compile failed: %v", err)
	}
	out, err := expr.Eval(nil, map[string]interface{}{"greet": "Hi"})
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(string) != "Hi" {
		t.Fatalf("expected Hi, got %v", out)
	}
}

func TestNewAPI_TimeCallablesStableWithinEval(t *testing.T) {
	out, err := evalNew(t, `{"now": $now(), "delay": $sum([1..10000]), "later": $now()}.(now = later)`, nil, nil, nil)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(bool) != true {
		t.Fatalf("expected true, got %v", out)
	}
}

func TestCompiler_CompileWithBaseVarsExts(t *testing.T) {
	comp, err := NewCompiler(
		map[string]interface{}{"greet": "Hello"},
		map[string]Extension{
			"twice": {Func: func(x float64) float64 { return x * 2 }},
		},
	)
	if err != nil {
		t.Fatalf("NewCompiler failed: %v", err)
	}

	expr, err := comp.Compile("$greet & ' ' & $twice($.n)")
	if err != nil {
		t.Fatalf("Compiler.Compile failed: %v", err)
	}
	out, err := expr.Eval(map[string]interface{}{"n": 21}, nil)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(string) != "Hello 42" {
		t.Fatalf("expected Hello 42, got %v", out)
	}
}

func TestCompiler_MergeWithEvaluatorExtras(t *testing.T) {
	comp, err := NewCompiler(map[string]interface{}{"greet": "Hello"}, nil)
	if err != nil {
		t.Fatalf("NewCompiler failed: %v", err)
	}

	expr, err := comp.Compile("$greet")
	if err != nil {
		t.Fatalf("Compiler.Compile failed: %v", err)
	}
	out, err := expr.Eval(nil, map[string]interface{}{"greet": "Hi"})
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(string) != "Hi" {
		t.Fatalf("expected Hi, got %v", out)
	}
}
