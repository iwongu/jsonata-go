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
