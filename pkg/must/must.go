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

package must

import (
	"time"

	"github.com/open-policy-agent/opa/ast"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Must panics if the error is set.
func Must(err error) {
	if err != nil {
		panic(err.Error())
	}
}

// Check panics if b is false. This is similar to Bool, but intended for
// use by runtime checks, rather than by functions that return (bool, error).
func Check(b bool, err error) {
	if !b {
		panic(err.Error())
	}
}

// Bytes panics if the error is set, otherwise returns b.
func Bytes(b []byte, err error) []byte {
	if err != nil {
		panic(err.Error())
	}

	return b
}

// Bool panics if the error is set, otherwise returns b.
func Bool(b bool, err error) bool {
	if err != nil {
		panic(err.Error())
	}

	return b
}

// Duration panics if the error is set, otherwise returns d.
func Duration(d time.Duration, err error) time.Duration {
	if err != nil {
		panic(err.Error())
	}

	return d
}

// GroupVersion panics if the error is set, otherwise returns b.
func GroupVersion(gv schema.GroupVersion, err error) schema.GroupVersion {
	if err != nil {
		panic(err.Error())
	}

	return gv
}

// String panics if the error is set, otherwise returns s.
func String(s string, err error) string {
	if err != nil {
		panic(err.Error())
	}

	return s
}

// StringSlice panics if the error is set, otherwise returns s.
func StringSlice(s []string, err error) []string {
	if err != nil {
		panic(err.Error())
	}

	return s
}

// Int panics if the error is set, otherwise returns i.
func Int(i int, err error) int {
	if err != nil {
		panic(err.Error())
	}

	return i
}

// Unstructured ...
func Unstructured(u *unstructured.Unstructured, err error) *unstructured.Unstructured {
	if err != nil {
		panic(err.Error())
	}

	return u
}

// Module ...
func Module(m *ast.Module, err error) *ast.Module {
	if err != nil {
		panic(err.Error())
	}

	return m
}
