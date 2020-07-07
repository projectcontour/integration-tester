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

package fixture

import (
	"fmt"

	"github.com/projectcontour/integration-tester/pkg/doc"
	"github.com/projectcontour/integration-tester/pkg/filter"
	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/utils"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	sigyaml "sigs.k8s.io/yaml"
)

// Fixture captures a single Kubernetes object that can be used as
// a test fixture. The fixture is stored as a YAML string so that
// is can be succinctly copied and losslessly rewritten.
type Fixture []byte

// AsNode returns a copy of the fixture as a yaml.RNode.
func (f Fixture) AsNode() *yaml.RNode {
	return yaml.MustParse(string(f))
}

// AsUnstructured returns a copy of the fixture as a unstructured.Unstructured object.
func (f Fixture) AsUnstructured() *unstructured.Unstructured {
	jsonBytes := must.Bytes(sigyaml.YAMLToJSON(f))

	resource, _, err := unstructured.UnstructuredJSONScheme.Decode(jsonBytes, nil, nil)
	if err != nil {
		panic(fmt.Sprintf("failed to decode JSON: %s", err))
	}

	return resource.(*unstructured.Unstructured)
}

// Rename updates the `metadata.name` and `metadata.namespace`
// fields of the fixture. YAML anchors are preserved so if the
// updated values of these fields will continue to be propagated to
// aliases.
func (f Fixture) Rename(newName string) (Fixture, error) {
	resource := f.AsNode()

	ns, name := utils.SplitObjectName(newName)

	_, err := resource.Pipe(&filter.Rename{
		Name:      name,
		Namespace: ns,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to rename object: %w", err)
	}

	return Fixture(resource.MustString()), nil
}

// AddFromFile parses all the YAML objects from the given file and
// stores them in the default fixture set.
func AddFromFile(filePath string) error {
	d, err := doc.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read %q`: %w", filePath, err)
	}

	for i, p := range d.Parts {
		ftype, err := p.Decode()
		if err != nil {
			return fmt.Errorf(
				"failed to parse document fragment %d: %w", i, err)
		}

		if ftype == doc.FragmentTypeObject {
			Set.Insert(
				KeyFor(p.Object()),
				Fixture(utils.CopyBytes(p.Bytes)),
			)
		}
	}

	return nil
}
