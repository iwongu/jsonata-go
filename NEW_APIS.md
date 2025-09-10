# New APIs (CompiledExpression/Evaluator)

Concise overview of the newer JSONata-Go APIs in `jsonata_new.go`. These provide immutable compiled expressions and lightweight evaluators without a default global registry.

## Quick start

```go
package main

import (
    "fmt"

    jsonata "github.com/iwongu/jsonata-go"
)

func main() {
    // Compile once; share across goroutines
    ce, err := jsonata.CompileExpression("$twice($.x)")
    if err != nil { panic(err) }

    // Attach extensions immutably (copy-on-write)
    ce, err = ce.WithExts(map[string]jsonata.Extension{
        "twice": {Func: func(n float64) float64 { return n * 2 }},
    })
    if err != nil { panic(err) }

    // Per-goroutine evaluator
    ev := ce.NewEvaluator()

    out, err := ev.Eval(map[string]interface{}{"x": 21})
    if err != nil { panic(err) }

    fmt.Println(out) // 42
}
```

## API surface

- `CompileExpression(expr string) (*CompiledExpression, error)` — compile once, immutable, shareable.
- `(c *CompiledExpression) WithExts(map[string]Extension) (*CompiledExpression, error)` — add extensions (copy-on-write).
- `(c *CompiledExpression) WithVars(map[string]interface{}) (*CompiledExpression, error)` — add variables (copy-on-write).
- `(c *CompiledExpression) NewEvaluator() *Evaluator` — create evaluator.
- `(e *Evaluator) RegisterExts(map[string]Extension) error` — evaluator-local extensions.
- `(e *Evaluator) RegisterVars(map[string]interface{}) error` — evaluator-local variables.
- `(e *Evaluator) Eval(data interface{}) (interface{}, error)` — evaluate with `data` bound to `$`.

## Concurrency model

- `CompiledExpression`: immutable and goroutine-safe.
- `Evaluator`: not goroutine-safe; create one per goroutine.

## Registry and scoping

- No default global registry. Choose:
  - Bind at compile-time via `WithExts`/`WithVars` (shared by all evaluators from that compiled expression), or
  - Bind per evaluator via `RegisterExts`/`RegisterVars`.
- Built-ins are available. Time callables `$millis` and `$now` are bound per evaluation with the current timestamp.

## Differences vs legacy API (`jsonata.go`)

- Legacy relies on package-level `RegisterExts`/`RegisterVars` (global mutable registry).
- New API separates compile (shareable) from evaluate (per goroutine) and makes bindings explicit.

## Notes

- Undefined result returns `ErrUndefined`.
- Nil pointers yield `nil` results.
- Names for variables/extensions must be valid identifiers (letters, digits, underscore).

## Additional examples

- Add variables at compile time:

```go
ce, _ := jsonata.CompileExpression("$greet & ' ' & $.name")
ce, _ = ce.WithVars(map[string]interface{}{"greet": "Hello"})
ev := ce.NewEvaluator()
out, _ := ev.Eval(map[string]interface{}{"name": "Ada"})
// out == "Hello Ada"
```

- Add evaluator-local state:

```go
ev := ce.NewEvaluator()
ev.RegisterVars(map[string]interface{}{"limit": 10})
ev.RegisterExts(map[string]jsonata.Extension{
    "cap": {Func: func(n float64, limit float64) float64 {
        if n > limit { return limit }
        return n
    }},
})
```

See `README.md` and `jsonata.go` for the legacy API and general usage.
