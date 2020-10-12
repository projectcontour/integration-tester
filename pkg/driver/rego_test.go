// Copyright  Project Contour Authors
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
	"testing"

	"github.com/projectcontour/integration-tester/pkg/result"
	"github.com/projectcontour/integration-tester/pkg/utils"

	"github.com/open-policy-agent/opa/ast"
	"github.com/open-policy-agent/opa/rego"
	"github.com/open-policy-agent/opa/storage"
	"github.com/open-policy-agent/opa/storage/inmem"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func parse(t *testing.T, text string) (*ast.Module, RegoOpt) {
	t.Helper()

	m, err := ast.ParseModule("test", text)
	if err != nil {
		t.Fatalf("failed to parse module: %s", err)
	}

	// (*CheeckDriver)Eval() doesn't compile anything, so we
	// need to pass in a compiler with pre-loaded modules.
	c := ast.NewCompiler()
	if c.Compile(map[string]*ast.Module{"test": m}); c.Failed() {
		t.Fatalf("failed to compile module: %s", c.Errors)
	}

	return m, rego.Compiler(c)
}

func TestQueryNoResult(t *testing.T) {
	r := NewRegoDriver()

	results, err := r.Eval(parse(t, `
package test

foo := true

error[msg] { not foo; msg = "this is the error"}
error[msg] { not foo; msg = "this is the second error"}
fatal[msg] { input.bar; msg = "this is the fatal error"}
`))

	require.NoError(t, err)

	// None of the rules are true, so their result set should be empty.
	assert.ElementsMatch(t, []result.Result{}, results)
}

func TestQueryStringResult(t *testing.T) {
	r := NewRegoDriver()

	results, err := r.Eval(parse(t, `
package test

error[msg] { msg = "this is the error"}
error[msg] { msg = "this is the second error"}
fatal[msg] { msg = "this is the fatal error"}
`))

	require.NoError(t, err)

	expected := []result.Result{{
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"error\"",
			"this is the error",
		),
	}, {
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"error\"",
			"this is the second error",
		),
	}, {
		Severity: result.SeverityFatal,
		Message: utils.JoinLines(
			"raised predicate \"fatal\"",
			"this is the fatal error"),
	}}

	assert.ElementsMatch(t, expected, results)
}

func TestQueryMapResult(t *testing.T) {
	r := NewRegoDriver()

	results, err := r.Eval(parse(t, `
package test

error [{"msg": msg, "foo": "bar"}] { msg = "this is the nested error"}
`))

	require.NoError(t, err)

	expected := []result.Result{{
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"error\"",
			"this is the nested error"),
	}}

	assert.ElementsMatch(t, expected, results)
}

func TestQueryBoolResult(t *testing.T) {
	r := NewRegoDriver()

	results, err := r.Eval(parse(t, `
package test

error  { msg = "this error doesn't appear"}
`))

	require.NoError(t, err)

	expected := []result.Result{{
		Severity: result.SeverityError,
		Message:  "raised predicate \"error\"",
	}}

	assert.ElementsMatch(t, expected, results)
}

func TestQueryStringSliceResult(t *testing.T) {
	r := NewRegoDriver()

	results, err := r.Eval(parse(t, `
package test

error [msg] {
  msg := [
	"message one",
	"message two",
  ]
}

error [msg] {
  msg := [
	"message three",
	"message four",
  ]
}

`))

	require.NoError(t, err)

	expected := []result.Result{{
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"error\"",
			"message one",
			"message two",
		),
	}, {
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"error\"",
			"message three",
			"message four",
		),
	}}

	assert.ElementsMatch(t, expected, results)
}

func TestQueryUntypedResult(t *testing.T) {
	r := NewRegoDriver()

	results, err := r.Eval(parse(t, `
package foo

sites := [
    {"count": 1},
    {"count": 2},
    {"count": 3},
]

error[num] { num := sites[_].count }
`))

	require.NoError(t, err)

	expected := []result.Result{{
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"error\"",
			"unhandled result value type 'json.Number'",
			"1\n", // Trailing newline because YAML.
		),
	}, {
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"error\"",
			"unhandled result value type 'json.Number'",
			"2\n", // Trailing newline because YAML.
		),
	}, {
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"error\"",
			"unhandled result value type 'json.Number'",
			"3\n", // Trailing newline because YAML.
		),
	}}

	assert.ElementsMatch(t, expected, results)
}

func TestQueryResultResult(t *testing.T) {
	r := NewRegoDriver()

	results, err := r.Eval(parse(t, `
package foo

check[result] {
    result := {
	"result": "Skip",
	"msg": "skipped message here",
    }
}

check_something_else[result] {
    result := {
        "result": "Error",
        "msg": "error message here",
    }
}

check_something_else[result] {
    result := {
        "result": "Pass",
        "msg": "this check passed",
    }
}
`))

	require.NoError(t, err)

	expected := []result.Result{{
		Severity: result.SeveritySkip,
		Message: utils.JoinLines(
			"raised predicate \"check\"",
			"skipped message here",
		),
	}, {
		Severity: result.SeverityError,
		Message: utils.JoinLines(
			"raised predicate \"check_something_else\"",
			"error message here",
		),
	}, {
		Severity: result.SeverityPass,
		Message: utils.JoinLines(
			"raised predicate \"check_something_else\"",
			"this check passed"),
	}}

	assert.ElementsMatch(t, expected, results)
}

func TestStorePathItem(t *testing.T) {
	// Use the underlying Rego driver type so we can directly access the Store.
	r := &regoDriver{
		store: inmem.New(),
	}

	ctx := context.TODO()

	// Creating the same path twice is not an error.
	assert.NoError(t, r.StorePath("/test/path/one"))
	assert.NoError(t, r.StorePath("/test/path/one"))

	storedValue := map[string]interface{}{
		"item": map[string]interface{}{
			"first":  "one",
			"second": "two",
		},
	}

	read := func(where string) (interface{}, error) {
		txn := storage.NewTransactionOrDie(ctx, r.store)
		defer r.store.Abort(ctx, txn)

		// Ensure that we can read it back.
		return r.store.Read(ctx, txn, storage.MustParsePath(where))
	}

	// Store an item.
	assert.NoError(t, r.StoreItem("/test/path/two", storedValue))

	// Ensure that we can read it back.
	val, err := read("/test/path/two")
	require.NoError(t, err, "reading store path %q", "/test/path/two")
	assert.Equal(t, storedValue, val)

	// Now re-store the path.
	assert.NoError(t, r.StorePath("/test/path/two"))

	// Ensure that extending the path didn't nuke the value
	val, err = read("/test/path/two")
	require.NoError(t, err, "reading store path %q", "/test/path/two")
	assert.Equal(t, storedValue, val)

	updatedValue := map[string]interface{}{
		"item": map[string]interface{}{
			"first":  "one",
			"second": "two",
			"third":  map[string]interface{}{},
		},
	}

	// Now store a path that traverses the existing item.
	assert.NoError(t, r.StorePath("/test/path/two/item/third"))

	// Ensure that extending the path didn't nuke the value, but created a new field.
	val, err = read("/test/path/two")
	require.NoError(t, err, "reading store path %q", "/test/path/two")
	assert.Equal(t, updatedValue, val)
}

func TestStoreRemoveItem(t *testing.T) {
	// Use the underlying Rego driver type so we can directly access the Store.
	r := &regoDriver{
		store: inmem.New(),
	}

	ctx := context.TODO()

	//nolint(unparam)
	read := func(where string) (interface{}, error) {
		txn := storage.NewTransactionOrDie(ctx, r.store)
		defer r.store.Abort(ctx, txn)

		// Ensure that we can read it back.
		return r.store.Read(ctx, txn, storage.MustParsePath(where))
	}

	assert.NoError(t, r.StorePath("/test/path/one"))

	// Ensure that we can read it back.
	_, err := read("/test/path/one")
	require.NoError(t, err, "reading store path %q", "/test/path/one")

	assert.NoError(t, r.RemovePath("/test/path/one"))

	// Ensure that it is gone can read it back.
	_, err = read("/test/path/one")
	require.True(t, storage.IsNotFound(err), "error is %s", err)

	assert.True(t, storage.IsNotFound(r.RemovePath("/no/such/path")))
}
