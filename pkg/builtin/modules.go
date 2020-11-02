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

package builtin

import (
	"fmt"
	"strings"

	"github.com/open-policy-agent/opa/ast"
	"github.com/projectcontour/integration-tester/pkg/must"
)

// CompileModules compiles all the built-in Rego files. We require that
// each file has a unique module name.
func CompileModules() (map[string]*ast.Module, error) {
	modmap := map[string]*ast.Module{}

	for _, a := range AssetNames() {
		if !strings.HasSuffix(a, ".rego") {
			continue
		}

		str := string(must.Bytes(Asset(a)))
		m := must.Module(ast.ParseModule(a, str))

		if _, ok := modmap[a]; ok {
			return nil, fmt.Errorf("duplicate builtin Rego module asset %q", a)
		}

		modmap[a] = m
	}

	return modmap, nil
}
