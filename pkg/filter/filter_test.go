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

package filter

import (
	"fmt"
	"strings"
	"testing"

	"github.com/projectcontour/integration-tester/pkg/version"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	sigyaml "sigs.k8s.io/yaml"
)

func TestSpecialOpsFilter(t *testing.T) {
	specialOps := SpecialOpsFilter{}
	rn, err := yaml.MustParse(`
apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: httpbin
$special: special value
`).Pipe(&specialOps)

	require.NoError(t, err)
	assert.Equal(t, specialOps.Ops, map[string]interface{}{
		"$special": "special value",
	})

	// Verify that we removed the special node.
	assert.Equal(t,
		strings.TrimSpace(rn.MustString()),
		strings.TrimSpace(`apiVersion: projectcontour.io/v1
kind: HTTPProxy
metadata:
  name: httpbin`))
}

func TestSpecialOpsFilterFixture(t *testing.T) {
	specialOps := SpecialOpsFilter{
		Decoders: map[string]yaml.Unmarshaler{},
	}

	type Fixture struct {
		As string
	}

	fixtureAs := Fixture{}

	specialOps.Decoders["$apply"] = UnmarshalFunc(func(n *yaml.Node) error {
		var as struct{ Fixture Fixture }
		var str string

		if err := n.Decode(&as); err == nil {
			specialOps.Ops["$apply"] = as.Fixture
			fixtureAs = as.Fixture
			return nil
		}

		if err := n.Decode(&str); err == nil {
			specialOps.Ops["$apply"] = str
			return nil
		}

		return fmt.Errorf("unable to decode YAML field %q", "$apply")
	})

	rn := yaml.MustParse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: httpbin
$apply:
  fixture:
    as: bar/foo
`)

	_, err := rn.Pipe(&specialOps)
	assert.NoError(t, err)

	_, ok := specialOps.Ops["$apply"].(Fixture)
	assert.True(t, ok, "$apply element is not a Fixture struct")
	assert.Equal(t, "bar/foo", fixtureAs.As)

	rn = yaml.MustParse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: httpbin
$apply: fixture
`)

	_, err = rn.Pipe(&specialOps)
	assert.NoError(t, err)

	_, ok = specialOps.Ops["$apply"].(string)
	assert.True(t, ok, "$apply element is not a string")
	assert.Equal(t, "fixture", specialOps.Ops["$apply"].(string))

}

func TestMetaInjectionFilter(t *testing.T) {
	rn := yaml.MustParse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: httpbin
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: httpbin
  template:
    metadata:
      labels:
        app.kubernetes.io/name: httpbin
    spec:
      containers:
      - image: docker.io/kennethreitz/httpbin
`)

	i := &MetaInjectionFilter{
		RunID:     "test-run-id",
		ManagedBy: version.Progname,
	}

	_, err := rn.Pipe(i)
	require.NoError(t, err)

	wanted := yaml.MustParse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: httpbin
  labels:
    app.kubernetes.io/managed-by: integration-tester
  annotations:
    integration-tester/run-id: test-run-id
spec:
  replicas: 1
  selector:
    matchLabels:
      app.kubernetes.io/name: httpbin
  template:
    metadata:
      labels:
        app.kubernetes.io/name: httpbin
        app.kubernetes.io/managed-by: integration-tester
      annotations:
        integration-tester/run-id: test-run-id
    spec:
      containers:
      - image: docker.io/kennethreitz/httpbin
`)

	assert.Equal(t, rn.MustString(), wanted.MustString())
}

func TestRenameObject(t *testing.T) {
	orig := yaml.MustParse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: first-name
`)

	_, err := orig.Pipe(Rename{Name: "second-name", Namespace: "ns"})
	require.NoError(t, err)

	wanted := yaml.MustParse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: second-name
  namespace: ns
`)

	assert.Equal(t, wanted.MustString(), orig.MustString())
}

func TestRenameObjectWithAnchor(t *testing.T) {
	toJSON := func(rn *yaml.RNode) string {
		jsonBytes, err := sigyaml.YAMLToJSON([]byte(rn.MustString()))
		require.NoError(t, err)
		return string(jsonBytes)
	}

	first := yaml.MustParse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: &anchor first-name
  namespace: *anchor
`)

	_, err := first.Pipe(Rename{Name: "second-name"})
	require.NoError(t, err)

	wanted := yaml.MustParse(`
apiVersion: apps/v1
kind: Deployment
metadata:
  name: &anchor second-name
  namespace: *anchor
`)

	assert.Equal(t, wanted.MustString(), first.MustString())

	assert.Equal(t,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"second-name","namespace":"second-name"}}`,
		toJSON(first))

	assert.Equal(t,
		`{"apiVersion":"apps/v1","kind":"Deployment","metadata":{"name":"second-name","namespace":"second-name"}}`,
		toJSON(wanted))
}
