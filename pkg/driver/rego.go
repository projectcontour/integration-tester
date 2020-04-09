// Copyright 2020 VMware, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License"); you may
// not use this file except in compliance with the License.  You may obtain
// a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
// WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.  See the
// License for the specific language governing permissions and limitations
// under the License.

package driver

import (
	"context"
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/result"
	"github.com/projectcontour/integration-tester/pkg/utils"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/open-policy-agent/opa/topdown"
	"sigs.k8s.io/yaml"
)

// RegoOpt is a convenience type alias.
type RegoOpt = func(*rego.Rego)

// RegoTracer is a tracer for check execution.
type RegoTracer interface {
	topdown.Tracer
	Write()
}

type defaultTracer struct {
	*topdown.BufferTracer
	writer io.Writer
}

func (d *defaultTracer) Write() {
	topdown.PrettyTrace(d.writer, *d.BufferTracer)
}

var _ RegoTracer = &defaultTracer{}

// NewRegoTracer returns a new RegoTracer that traces to w.
func NewRegoTracer(w io.Writer) RegoTracer {
	return &defaultTracer{
		BufferTracer: topdown.NewBufferTracer(),
		writer:       w,
	}
}

// RegoDriver is a driver for running Rego policy checks.
type RegoDriver interface {
	// Eval evaluates the given module and returns and check results.
	Eval(*ast.Module, ...RegoOpt) ([]result.Result, error)

	Trace(RegoTracer)

	// StoreItem stores the value at the given path in the Rego data document.
	StoreItem(string, interface{}) error

	// StorePath creates the given path in the Rego data document.
	StorePath(where string) error

	// RemovePath remove any object at the given path in the Rego data document.
	RemovePath(where string) error
}

// NewRegoDriver creates a new RegoDriver that evaluates checks
// written in Rego.
//
// See https://www.openpolicyagent.org/docs/latest/policy-language/
func NewRegoDriver() RegoDriver {
	return &regoDriver{
		store: inmem.New(),
	}
}

var _ RegoDriver = &regoDriver{}

type regoDriver struct {
	store  storage.Store
	tracer RegoTracer
}

func (r *regoDriver) Trace(tracer RegoTracer) {
	r.tracer = tracer
}

// StoreItem stores the value at the given Rego store path.
func (r *regoDriver) StoreItem(where string, what interface{}) error {
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, r.store, storage.WriteParams)

	path := storage.MustParsePath(where)

	err := r.store.Write(ctx, txn, storage.ReplaceOp, path, what)
	if storage.IsNotFound(err) {
		err = r.store.Write(ctx, txn, storage.AddOp, path, what)
	}

	if err != nil {
		r.store.Abort(ctx, txn)
		return err
	}

	if err := r.store.Commit(ctx, txn); err != nil {
		return err
	}

	return nil
}

// StorePath creates the given path in the Rego data document.
func (r *regoDriver) StorePath(where string) error {
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, r.store, storage.WriteParams)

	var currentPath storage.Path

	for _, p := range storage.MustParsePath(where) {
		currentPath = append(currentPath, p)

		_, err := r.store.Read(ctx, txn, currentPath)
		switch {
		case err == nil:
			// If the read succeeded, there was an element.
			continue
		case storage.IsNotFound(err):
			// If the path element isn't there, cover it with am empty node.
			val := map[string]interface{}{}
			if err := r.store.Write(ctx, txn, storage.AddOp, currentPath, val); err != nil {
				r.store.Abort(ctx, txn)
				return err
			}
		default:
			// Any other error, abort and propagate it.
			r.store.Abort(ctx, txn)
			return err
		}
	}

	if err := r.store.Commit(ctx, txn); err != nil {
		return err
	}

	return nil
}

// RemovePath removes the given path in the Rego data document.
func (r *regoDriver) RemovePath(where string) error {
	ctx := context.Background()
	txn := storage.NewTransactionOrDie(ctx, r.store, storage.WriteParams)

	if err := r.store.Write(ctx, txn, storage.RemoveOp, storage.MustParsePath(where), nil); err != nil {
		r.store.Abort(ctx, txn)
		return err
	}

	if err := r.store.Commit(ctx, txn); err != nil {
		return err
	}

	return nil
}

// Eval evaluates checks in the given module.
func (r *regoDriver) Eval(m *ast.Module, opts ...RegoOpt) ([]result.Result, error) {
	// Find the unique set of assertion rules to query.
	ruleNames := findAssertionRules(m)
	checkResults := make([]result.Result, 0, len(ruleNames))

	for _, name := range ruleNames {
		// The package path will be an absolute path through the
		// data document, so to convert that into the package
		// name, we trim the leading "data." component. We need
		// the literal package name of the module in the query
		// context so names resolve correctly.
		pkg := strings.TrimPrefix(m.Package.Path.String(), "data.")

		// NOTE(jpeach): we assume that the caller has
		// passed a compiler in the options and that if
		// the given module hasn't already been compiled,
		// the caller also passed a ParsedModule option.

		options := []RegoOpt{
			// Scope the query to the current module package.
			rego.Package(pkg),
			// Query for the result of this named rule.
			rego.Query(queryForRuleName(name)),
			rego.Store(r.store),
		}

		options = append(options, opts...)

		if r.tracer != nil {
			options = append(options, rego.Tracer(r.tracer))
		}

		regoObj := rego.New(options...)
		resultSet, err := regoObj.Eval(context.Background())

		if r.tracer != nil {
			r.tracer.Write()
		}

		// If this was a builtin error, we can return it as a
		// result. Builtins that fail are typically those that
		// access external resources (e.g. HTTP), in which case
		// the failure can be considered part of the test, not
		// part of the driver.
		if top := utils.AsRegoTopdownErr(err); top != nil &&
			top.Code == topdown.BuiltinErr {
			checkResults = append(checkResults,
				result.Result{
					Severity: result.SeverityError,
					Message:  top.Error(),
				})

			// Consume the error.
			err = nil
		}

		// If we didn't consume the error, puke it up the stack.
		if err != nil {
			return nil, err
		}

		// In each result, the Text is the expression that we
		// queried, and value is one or more bound messages.
		for _, r := range resultSet {
			for _, e := range r.Expressions {
				if r := extractResult(e); r != nil {
					checkResults = append(checkResults, *r)
				}
			}
		}

	}

	return checkResults, nil
}

// extractResult examines a rego.ExpressionValue to find the result
// (message) of a rule that we queried . A Rego query has an optional
// key term that can be of any type. In most cases, the term will be
// a string, like this:
// 	`error[msg]{ ... }`
// but it could be anything. For example, a map like this:
// 	`error[{"msg": "foo", "sev": "bad"}]{ ... }`
//
// So here, we follow the example of conftest and accept a key term
// that is either a string or a map with a string-valued key names
// "msg". In the future, we could accept other types, but
//
// See also https://github.com/instrumenta/conftest/pull/243.
func extractResult(expr *rego.ExpressionValue) *result.Result {
	res := result.Result{
		Severity: severityForRuleName(expr.Text),
		Message:  fmt.Sprintf("raised predicate %q", expr.Text),
	}

	switch value := expr.Value.(type) {
	case bool:
		// This might be a boolean if the rule was this:
		//	`error { ... }`
		//
		// Rego only returns the results of boolean rules
		// if the rule was true, so the value of the bool
		// result doesn't matter. We just know there's no
		// message.
		return &res

	case string:
		// This might be a string if the rule was this:
		//	`error = msg {
		//	 	...
		//		msg := "this is a failing thing"
		//	}`
		//
		res.Message = utils.JoinLines(res.Message, value)
		return &res

	case []interface{}:
		// Extract messages from the value slice. The reason there is
		// a slice is that there can be many matching cases for this
		// rule and the query evaluates them all simultaneously. Each
		// matching case might emit a message.

		if len(value) == 0 {
			return nil
		}

		for _, v := range value {
			// First, see if the value is a slice of strings. We
			// do this manually, because there's not enough type
			// information for the `[]string` case to match below.
			if s, ok := utils.AsStringSlice(v); ok {
				res.Message = utils.JoinLines(res.Message,
					utils.JoinLines(s...))
				continue
			}

			switch value := v.(type) {
			case string:
				res.Message = utils.JoinLines(res.Message, value)
			case []string:
				res.Message = utils.JoinLines(res.Message,
					utils.JoinLines(value...))
			case map[string]interface{}:
				if _, ok := value["msg"]; ok {
					if m, ok := value["msg"].(string); ok {
						res.Message = utils.JoinLines(res.Message, m)
					}
				}
			default:
				log.Printf("slice value of non-string %T: %v", value, value)
			}
		}

		return &res

	default:
		// We don't know how to deal with this kind of result, so just puke it out as YAML.
		res.Message = utils.JoinLines(res.Message,
			fmt.Sprintf("unhandled result value type '%T'", expr.Value),
			string(must.Bytes(yaml.Marshal(expr.Value))),
		)

		return &res
	}
}
