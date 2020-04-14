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

package test

import (
	"github.com/projectcontour/integration-tester/pkg/builtin"
	"github.com/projectcontour/integration-tester/pkg/driver"
	"github.com/projectcontour/integration-tester/pkg/must"

	"github.com/open-policy-agent/opa/ast"
)

// DefaultObjectCheckForOperation returns a built-in default check
// for applying Kubernetes objects.
func DefaultObjectCheckForOperation(op driver.ObjectOperationType) *ast.Module {
	var data []byte
	var name string

	switch op {
	case driver.ObjectOperationUpdate:
		name = "pkg/builtin/objectUpdateCheck.rego"
	case driver.ObjectOperationDelete:
		name = "pkg/builtin/objectDeleteCheck.rego"
	}

	data = must.Bytes(builtin.Asset(name))
	return must.Module(ast.ParseModule(name, string(data)))
}
