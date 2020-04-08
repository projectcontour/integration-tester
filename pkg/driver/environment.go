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
	"fmt"

	"github.com/projectcontour/integration-tester/pkg/doc"
	"github.com/projectcontour/integration-tester/pkg/filter"
	"github.com/projectcontour/integration-tester/pkg/fixture"
	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/version"

	"github.com/google/uuid"
	"github.com/open-policy-agent/opa/ast"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/yaml"
	sigyaml "sigs.k8s.io/yaml"
)

// Environment holds metadata that describes the context of a test.
type Environment interface {
	// UniqueID returns a unique identifier for this Environment instance.
	UniqueID() string

	// HydrateObject ...
	HydrateObject(objData []byte) (*Object, error)
}

// NewEnvironment returns a new Environment.
func NewEnvironment() Environment {
	return &environ{
		uid: uuid.New().String(),
	}
}

var _ Environment = &environ{}

type environ struct {
	uid string
}

// UniqueID returns a unique identifier for this Environment instance.
func (e *environ) UniqueID() string {
	return e.uid
}

// ObjectOperationType desscribes the type of operation to apply
// to this object. This is derived from the "$apply" pseudo-field.
type ObjectOperationType string

const (
	// ObjectOperationDelete indicates this object should be deleted.
	ObjectOperationDelete = "delete"
	// ObjectOperationUpdate indicates this object should be
	// updated (i.e created or patched).
	ObjectOperationUpdate = "update"
)

// Fixture is a marker to tell the Environment that a Kubernetes
// object is a fixture placeholder.
type Fixture struct {
	As string
}

// Object captures an Unstructured Kubernetes API object and its
// associated metadata.
//
// TODO(jpeach): this is a terrible name. Refactor this whole bizarre atrocity.
type Object struct {
	// Object is the object to apply.
	Object *unstructured.Unstructured

	// Check is a Rego check to run on the apply.
	Check *ast.Module

	// Operation specifies whether we are updating or deleting the object.
	Operation ObjectOperationType

	// Fixture specifies that we should replace this object with the corresponding fixture.
	Fixture *Fixture
}

func yamlToUnstructured(node *yaml.RNode) (*unstructured.Unstructured, error) {
	jsonBytes, err := sigyaml.YAMLToJSON([]byte(node.MustString()))
	if err != nil {
		return nil, err
	}

	resource, _, err := unstructured.UnstructuredJSONScheme.Decode(jsonBytes, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %s", err)
	}

	return resource.(*unstructured.Unstructured), nil
}

func matchFixture(resource *yaml.RNode) fixture.Fixture {
	u := must.Unstructured(yamlToUnstructured(resource))

	if match := fixture.Set.Match(u); match != nil {
		return match
	}

	return nil
}

// HydrateObject unmarshals YAML data into a unstructured.Unstructured
// object, applying any defaults and expanding templates.
func (e *environ) HydrateObject(objData []byte) (*Object, error) {
	// TODO(jpeach): before parsing YAML, apply Go template context.

	resource, err := yaml.Parse(string(objData))
	if err != nil {
		return nil, fmt.Errorf("failed to parse YAML object:%w", err)
	}

	// Filter out any special operations.
	ops := newSpecialOpsFilter()

	resource, err = resource.Pipe(ops)
	if err != nil {
		return nil, fmt.Errorf("special ops filtering: %w", err)
	}

	// Before we make any modifications to the object we just
	// parsed, check if we need to replace it with a fixture.
	if val, ok := ops.Ops["$apply"]; ok {
		if fix, ok := val.(Fixture); ok {
			match := matchFixture(resource)
			if match == nil {
				return nil, fmt.Errorf("failed to match fixture")
			}

			if fix.As != "" {
				match, err = match.Rename(fix.As)
				if err != nil {
					return nil, fmt.Errorf("failed to rename fixture object: %w", err)
				}
			}

			resource = match.AsNode()
		}
	}

	// Inject test metadata.
	resource, err = resource.Pipe(
		&filter.MetaInjectionFilter{RunID: e.UniqueID(), ManagedBy: version.Progname})
	if err != nil {
		return nil, fmt.Errorf("metadata injection failed: %w", err)
	}

	o := Object{
		Object:    &unstructured.Unstructured{},
		Operation: ObjectOperationUpdate,
	}

	for key, handler := range specialOpHandlers {
		what, ok := ops.Ops[key]
		if !ok {
			continue
		}

		if err := handler(what, &o); err != nil {
			return nil, err
		}
	}

	o.Object, err = yamlToUnstructured(resource)
	if err != nil {
		return nil, err
	}

	return &o, nil
}

func newSpecialOpsFilter() *filter.SpecialOpsFilter {
	// Filter out any special operations.
	ops := filter.SpecialOpsFilter{
		Decoders: map[string]yaml.Unmarshaler{},
	}

	ops.Decoders["$apply"] = filter.UnmarshalFunc(func(n *yaml.Node) error {
		var as struct{ Fixture Fixture }
		var str string

		// We support two syntaxes for fixtures:
		//	$apply: fixture
		// and
		//	$apply:
		//	  fixture:
		//	    as: some-other-name

		if err := n.Decode(&as); err == nil {
			ops.Ops["$apply"] = as.Fixture
			return nil
		}

		if err := n.Decode(&str); err == nil {
			switch str {
			case "fixture":
				ops.Ops["$apply"] = Fixture{}
			default:
				ops.Ops["$apply"] = str
			}

			return nil
		}

		return fmt.Errorf("unable to decode YAML field %q", "$apply")
	})

	return &ops
}

var specialOpHandlers = map[string]func(val interface{}, o *Object) error{
	"$check": func(val interface{}, o *Object) error {
		strval, ok := val.(string)
		if !ok {
			return fmt.Errorf(
				"failed to decode %q field: unexpected type %T",
				"$check", strval)
		}

		frag, err := doc.NewRegoFragment([]byte(strval))
		if err != nil {
			return err
		}

		o.Check = frag.Rego()
		return nil
	},

	"$apply": func(val interface{}, o *Object) error {
		switch what := val.(type) {
		case string:
			switch what {
			case "update":
				o.Operation = ObjectOperationUpdate
			case "delete":
				o.Operation = ObjectOperationDelete
			case "fixture":
				o.Operation = ObjectOperationUpdate
			default:
				return fmt.Errorf(
					"unsupported operation %q for %q field", what, "$apply")
			}
		case Fixture:
			o.Operation = ObjectOperationUpdate
		default:
			return fmt.Errorf(
				"failed to decode %q field: unexpected type %T",
				"$apply", what)
		}

		return nil
	},
}
