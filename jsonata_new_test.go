package jsonata

import (
	"fmt"
	"sync"
	"testing"
)

func TestCompileExpressionAndEval_Simple(t *testing.T) {
	ce, err := CompileExpression("1+2")
	if err != nil {
		t.Fatalf("CompileExpression failed: %v", err)
	}

	ev := ce.NewEvaluator()
	out, err := ev.Eval(nil)
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

func TestCompiledExpression_WithExtsAndEval(t *testing.T) {
	ce, err := CompileExpression("$twice(21)")
	if err != nil {
		t.Fatalf("CompileExpression failed: %v", err)
	}

	ce2, err := ce.WithExts(map[string]Extension{
		"twice": {Func: func(x float64) float64 { return x * 2 }},
	})
	if err != nil {
		t.Fatalf("WithExts failed: %v", err)
	}

	ev := ce2.NewEvaluator()
	out, err := ev.Eval(nil)
	if err != nil {
		t.Fatalf("Eval failed: %v", err)
	}
	if out.(float64) != 42 {
		t.Fatalf("expected 42, got %v", out)
	}
}

func TestEvaluator_ConcurrentEval(t *testing.T) {
	// Use a contextable builtin via function application to exercise per-call context.
	ce, err := CompileExpression("'HelloWorld' ~> $substring(5, 5)")
	if err != nil {
		t.Fatalf("CompileExpression failed: %v", err)
	}

	const goroutines = 50
	const iterations = 20

	var wg sync.WaitGroup
	errs := make(chan error, goroutines*iterations)
	wg.Add(goroutines)

	for g := 0; g < goroutines; g++ {
		go func() {
			defer wg.Done()
			ev := ce.NewEvaluator()
			for i := 0; i < iterations; i++ {
				out, err := ev.Eval(nil)
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

func evalNew(t *testing.T, expr string, input interface{}, vars map[string]interface{}, exts map[string]Extension) (interface{}, error) {
	ce, err := CompileExpression(expr)
	if err != nil {
		t.Fatalf("CompileExpression failed: %v", err)
	}
	if vars != nil {
		ce, err = ce.WithVars(vars)
		if err != nil {
			t.Fatalf("WithVars failed: %v", err)
		}
	}
	if exts != nil {
		ce, err = ce.WithExts(exts)
		if err != nil {
			t.Fatalf("WithExts failed: %v", err)
		}
	}
	ev := ce.NewEvaluator()
	return ev.Eval(input)
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
	ce, err := CompileExpression("$greet")
	if err != nil {
		t.Fatalf("CompileExpression failed: %v", err)
	}
	ce, err = ce.WithVars(map[string]interface{}{"greet": "Hello"})
	if err != nil {
		t.Fatalf("WithVars failed: %v", err)
	}
	ev := ce.NewEvaluator()
	if err := ev.RegisterVars(map[string]interface{}{"greet": "Hi"}); err != nil {
		t.Fatalf("RegisterVars failed: %v", err)
	}
	out, err := ev.Eval(nil)
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
