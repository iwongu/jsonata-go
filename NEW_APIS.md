# New APIs 

Overview of the new JSONata-Go APIs in `jsonata_new.go`. These provide goroutine safe compiled expressions. 

## Quick start

```go
package main

import (
    "fmt"

    jsonata "github.com/iwongu/jsonata-go"
)

func main() {
    // Configure compile-time variables and extensions
    compiler, err := jsonata.NewCompiler(
        map[string]interface{}{"greet": "Hello"},
        map[string]jsonata.Extension{
            "twice": {Func: func(n float64) float64 { return n * 2 }},
        },
    )
    if err != nil { panic(err) }

    // Compile once; share across goroutines
    expr, err := compiler.Compile("$greet & ' ' & $twice($.x)")
    if err != nil { panic(err) }

    // Per-call variables are passed to Eval
    out, err := expr.Eval(map[string]interface{}{"x": 21}, nil)
    if err != nil { panic(err) }

    fmt.Println(out) // Hello 42
}
```

## API surface

- `NewCompiler(vars map[string]interface{}, exts map[string]Extension) (*Compiler, error)` — create a configured compiler. can be a singleton.
- `(c *Compiler) Compile(expr string) (*Expression, error)` — parse/compile; result is immutable and shareable/cachaeable.
- `(e *Expression) Eval(data interface{}, vars map[string]interface{}) (interface{}, error)` — evaluate with `data` bound to `$` and optional per-call vars.

## Additional examples

- Add variables at compile time and call with data:

```go
compiler, _ := jsonata.NewCompiler(map[string]interface{}{"greet": "Hello"}, nil)
expr, _ := compiler.Compile("$greet & ' ' & $.name")
out, _ := expr.Eval(map[string]interface{}{"name": "Ada"}, nil)
// out == "Hello Ada"
```

- Provide per-call variables:

```go
expr, _ := compiler.Compile("$cap($.n)")
out, _ := expr.Eval(map[string]interface{}{"n": 12}, map[string]interface{}{"limit": 10})
```
