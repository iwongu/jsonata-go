// Copyright 2018 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package jsonata

import (
	"reflect"
	"time"

	"github.com/blues/jsonata-go/jparse"
)

// CompiledExpression is an immutable, thread-safe compiled JSONata expression
// that can spawn Evaluators.
type CompiledExpression struct {
	node         jparse.Node
	baseRegistry map[string]reflect.Value
}

// CompileExpression parses a JSONata expression and returns a CompiledExpression
// that can create Evaluators for execution.
func CompileExpression(expr string) (*CompiledExpression, error) {

	node, err := jparse.Parse(expr)
	if err != nil {
		return nil, err
	}

	// No global registry in the new API by default.
	return &CompiledExpression{node: node, baseRegistry: nil}, nil
}

// WithExts returns a new CompiledExpression with the provided extensions
// merged into the base registry (copy-on-write). The original is unchanged.
func (c *CompiledExpression) WithExts(exts map[string]Extension) (*CompiledExpression, error) {

	values, err := processExts(exts)
	if err != nil {
		return nil, err
	}

	old := c.baseRegistry
	newm := make(map[string]reflect.Value, len(old)+len(values))
	for k, v := range old {
		newm[k] = v
	}
	for k, v := range values {
		newm[k] = v
	}

	return &CompiledExpression{
		node:         c.node,
		baseRegistry: newm,
	}, nil
}

// WithVars returns a new CompiledExpression with the provided variables
// merged into the base registry (copy-on-write). The original is unchanged.
func (c *CompiledExpression) WithVars(vars map[string]interface{}) (*CompiledExpression, error) {

	values, err := processVars(vars)
	if err != nil {
		return nil, err
	}

	old := c.baseRegistry
	newm := make(map[string]reflect.Value, len(old)+len(values))
	for k, v := range old {
		newm[k] = v
	}
	for k, v := range values {
		newm[k] = v
	}

	return &CompiledExpression{
		node:         c.node,
		baseRegistry: newm,
	}, nil
}

// NewEvaluator creates a new Evaluator for this compiled expression.
// Evaluators are intended to be used by a single goroutine.
func (c *CompiledExpression) NewEvaluator() *Evaluator {
	return &Evaluator{
		expression: c,
		extras:     make(map[string]reflect.Value),
	}
}

// Evaluator executes a compiled expression. It can be configured with
// per-evaluator variables and extensions via RegisterVars/RegisterExts.
// Evaluator is not goroutine-safe; create one per goroutine.
type Evaluator struct {
	expression *CompiledExpression
	extras     map[string]reflect.Value
}

// RegisterExts adds per-evaluator extensions. Not goroutine-safe.
func (e *Evaluator) RegisterExts(exts map[string]Extension) error {
	values, err := processExts(exts)
	if err != nil {
		return err
	}
	for k, v := range values {
		e.extras[k] = v
	}
	return nil
}

// RegisterVars adds per-evaluator variables. Not goroutine-safe.
func (e *Evaluator) RegisterVars(vars map[string]interface{}) error {
	values, err := processVars(vars)
	if err != nil {
		return err
	}
	for k, v := range values {
		e.extras[k] = v
	}
	return nil
}

// Eval evaluates the compiled expression with the provided input.
func (e *Evaluator) Eval(data interface{}) (interface{}, error) {
	input, ok := data.(reflect.Value)
	if !ok {
		input = reflect.ValueOf(data)
	}

	env := e.newEnv(input)
	result, err := eval(e.expression.node, input, env)
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

func (e *Evaluator) newEnv(input reflect.Value) *environment {
	tc := timeCallables(time.Now())

	// Size hint: $ + time callables + base + extras
	baseCount := len(e.expression.baseRegistry)
	env := newEnvironment(baseEnv, 1+len(tc)+baseCount+len(e.extras))

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
	for name, v := range e.expression.baseRegistry {
		if v.IsValid() && v.CanInterface() {
			if gc, ok := v.Interface().(*goCallable); ok {
				env.bind(name, reflect.ValueOf(gc.clone()))
				continue
			}
		}
		env.bind(name, v)
	}

	// Bind evaluator extras, cloning any goCallable
	for name, v := range e.extras {
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
