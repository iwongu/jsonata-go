// Copyright 2018 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package jsonata

import (
	"reflect"
	"time"

	"github.com/iwongu/jsonata-go/jparse"
)

// Compiler prepares compiled expressions with a predefined base registry
// of variables and extensions. Safe to share across goroutines.
type Compiler struct {
	baseRegistry map[string]reflect.Value
}

// NewCompiler creates a Compiler seeded with the provided variables and extensions.
func NewCompiler(vars map[string]interface{}, exts map[string]Extension) (*Compiler, error) {
	base := make(map[string]reflect.Value)

	if len(vars) > 0 {
		values, err := processVars(vars)
		if err != nil {
			return nil, err
		}
		for k, v := range values {
			base[k] = v
		}
	}

	if len(exts) > 0 {
		values, err := processExts(exts)
		if err != nil {
			return nil, err
		}
		for k, v := range values {
			base[k] = v
		}
	}

	if len(base) == 0 {
		base = nil
	}
	return &Compiler{baseRegistry: base}, nil
}

// Compile parses an expression and returns an Expression with the
// compiler's base registry bound. The returned expression is immutable
// and goroutine-safe.
func (c *Compiler) Compile(expr string) (*Expression, error) {
	node, err := jparse.Parse(expr)
	if err != nil {
		return nil, err
	}

	var merged map[string]reflect.Value
	if len(c.baseRegistry) > 0 {
		merged = make(map[string]reflect.Value, len(c.baseRegistry))
		for k, v := range c.baseRegistry {
			merged[k] = v
		}
	}

	return &Expression{node: node, baseRegistry: merged}, nil
}

// Expression is an immutable, thread-safe compiled JSONata expression.
// It can be evaluated concurrently by multiple goroutines.
type Expression struct {
	node         jparse.Node
	baseRegistry map[string]reflect.Value
}

// Eval evaluates the expression with the provided input and per-evaluation variables.
// vars may be nil. This method is safe for concurrent use across goroutines.
func (e *Expression) Eval(data interface{}, vars map[string]interface{}) (interface{}, error) {
	input, ok := data.(reflect.Value)
	if !ok {
		input = reflect.ValueOf(data)
	}

	// Prepare per-eval extras from vars
	var extraValues map[string]reflect.Value
	if len(vars) > 0 {
		values, err := processVars(vars)
		if err != nil {
			return nil, err
		}
		extraValues = values
	}

	env := e.newEnv(input, extraValues)
	result, err := eval(e.node, input, env)
	if err != nil {
		return nil, err
	}

	if !result.IsValid() {
		return nil, ErrUndefined
	}
	if !result.CanInterface() {
		return nil, err
	}
	if result.Kind() == reflect.Ptr && result.IsNil() {
		return nil, nil
	}
	return result.Interface(), nil
}

func (e *Expression) newEnv(input reflect.Value, extras map[string]reflect.Value) *environment {
	tc := timeCallables(time.Now())

	// Size hint: $ + time callables + base + extras
	baseCount := len(e.baseRegistry)
	env := newEnvironment(baseEnv, 1+len(tc)+baseCount+len(extras))

	env.bind("$", input)
	env.bindAll(tc)

	// Clone built-in callables from baseEnv into this evaluation environment
	if baseEnv != nil && baseEnv.symbols != nil {
		for name, v := range baseEnv.symbols {
			if v.IsValid() && v.CanInterface() {
				if gc, ok := v.Interface().(*goCallable); ok {
					env.bind(name, reflect.ValueOf(gc.clone()))
				}
			}
		}
	}

	// Bind base registry, cloning any goCallable
	for name, v := range e.baseRegistry {
		if v.IsValid() && v.CanInterface() {
			if gc, ok := v.Interface().(*goCallable); ok {
				env.bind(name, reflect.ValueOf(gc.clone()))
				continue
			}
		}
		env.bind(name, v)
	}

	// Bind per-eval extras, cloning any goCallable (unlikely for vars, but safe)
	for name, v := range extras {
		if v.IsValid() && v.CanInterface() {
			if gc, ok := v.Interface().(*goCallable); ok {
				env.bind(name, reflect.ValueOf(gc.clone()))
				continue
			}
		}
		env.bind(name, v)
	}

	return env
}
