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

package filter

import (
	"strings"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/fieldmeta"
	"sigs.k8s.io/kustomize/kyaml/yaml"
)

const (
	// LabelRunID is an annotation key to mark an object with
	// the unique ID of a test run.
	LabelRunID = "integration-tester/run-id"

	// LabelVersion is an annotation key to mark an object
	// with the version of the test harness that created it.
	LabelVersion = "integration-tester/version"

	// LabelManagedBy is a label key to mark an object as
	// managed by the test harness.
	LabelManagedBy = "app.kubernetes.io/managed-by"
)

// SpecialOpsFilter is a yaml.Filter that extracts top-level YAML keys
// whose name begins with `$`. These keys denote special operations
// that test drivers need to interpolate.
type SpecialOpsFilter struct {
	Ops map[string]interface{}

	Decoders map[string]yaml.Unmarshaler
}

// UnmarshalFunc is a yaml.Unmarshaler adapator.
type UnmarshalFunc func(value *yaml.Node) error

// UnmarshalYAML ...
func (u UnmarshalFunc) UnmarshalYAML(node *yaml.Node) error {
	return u(node)
}

// Filter runs the SpecialOpsFilter.
func (s *SpecialOpsFilter) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	s.Ops = make(map[string]interface{})
	keep := make([]*yaml.Node, 0, len(rn.Content()))

	// Starting as index 0, we have alternate nodes for YAML
	// field names and YAML field values. A special ops field
	// is any field whose name begins with '$'.
	for i := 0; i < len(rn.Content()); i = yaml.IncrementFieldIndex(i) {
		key := rn.Content()[i]
		val := rn.Content()[i+1]

		// If the field name isn't a string, then who knows
		// what we should do. Skip it.
		if !isStringNode(key) {
			keep = append(keep, key, val)
			continue
		}

		if !strings.HasPrefix(key.Value, "$") {
			keep = append(keep, key, val)
			continue
		}

		d, ok := s.Decoders[key.Value]
		if !ok {
			d = UnmarshalFunc(func(n *yaml.Node) error {
				var str string

				if err := n.Decode(&str); err != nil {
					return err
				}

				s.Ops[key.Value] = str
				return nil
			})
		}

		if err := d.UnmarshalYAML(val); err != nil {
			return nil, err
		}
	}

	rn.YNode().Content = keep
	return rn, nil
}

func isStringNode(n *yaml.Node) bool {
	return n.Kind == yaml.ScalarNode &&
		n.Tag == fieldmeta.String.Tag()
}

// MetaInjectionFilter injects ObjectMeta data into Kubernetes objects.
// Specifically, it labels objects with the ManagedBy string, and
// annotates with the RunID.
type MetaInjectionFilter struct {
	RunID     string
	ManagedBy string
}

var _ yaml.Filter = &MetaInjectionFilter{}

// Filter ...
func (m *MetaInjectionFilter) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	// First, inject the management label to the top object.
	if _, err := rn.Pipe(
		yaml.PathGetter{Create: yaml.MappingNode, Path: []string{"metadata", "labels"}},
		yaml.FieldSetter{Name: "app.kubernetes.io/managed-by", StringValue: m.ManagedBy},
	); err != nil {
		return nil, err
	}

	// Next, inject the management label to the template. This
	// ensures that the management label propagates down to child
	// objects.
	if n, err := rn.Pipe(
		yaml.PathGetter{Path: []string{"spec", "template", "metadata", "labels"}},
	); err == nil && n != nil {
		if _, err := rn.Pipe(
			yaml.PathGetter{Path: []string{"spec", "template", "metadata", "labels"}},
			yaml.FieldSetter{Name: "app.kubernetes.io/managed-by", StringValue: m.ManagedBy},
		); err != nil {
			return nil, err
		}
	}

	// Next, label the top level with the run ID.
	if _, err := rn.Pipe(
		yaml.PathGetter{Create: yaml.MappingNode, Path: []string{"metadata", "annotations"}},
		yaml.FieldSetter{Name: LabelRunID, StringValue: m.RunID},
	); err != nil {
		return nil, err
	}

	// Check whether this looks like an object that has a pod spec template.
	if c, err := rn.Pipe(
		yaml.PathGetter{Path: []string{"spec", "template", "spec", "containers"}},
	); c == nil || err != nil {
		return rn, err
	}

	// Since this object has a pod spec template, inject test metadata annotations into it.
	if _, err := rn.Pipe(
		yaml.PathGetter{Create: yaml.MappingNode, Path: []string{"spec", "template", "metadata", "annotations"}},
		yaml.FieldSetter{Name: LabelRunID, StringValue: m.RunID},
	); err != nil {
		return nil, err
	}

	return rn, nil

}

// Rename is a filter that rewrites the name of a Kubernetes object,
// i.e. it replaces the value of the `metadata.name` field.
type Rename struct {
	// Name is the new name of the object.
	Name string
	// Namespace is the new namespace of the object.
	Namespace string
}

// Filter applies the rename and returns rn.
func (r Rename) Filter(rn *yaml.RNode) (*yaml.RNode, error) {
	setNode := func(path []string, value string) error {
		// If there is an existing node, we will use that. This preserves anchors.
		name, err := rn.Pipe(yaml.PathGetter{Path: path})
		if err != nil {
			return err
		}

		if name != nil {
			// Only reset the value for scalar nodes. We don't want to
			// rewrite Alias nodes because there is no way to know whether
			// that is wanted or not.
			if name.YNode().Kind == yaml.ScalarNode {
				name.YNode().SetString(r.Name)
			}
			return nil
		}

		head := path[:len(path)-1]
		tail := path[len(path)-1]
		_, err = rn.Pipe(
			yaml.PathGetter{Create: yaml.MappingNode, Path: head},
			yaml.FieldSetter{Name: tail, StringValue: value},
		)

		return err

	}

	if err := setNode([]string{"metadata", "name"}, r.Name); err != nil {
		return nil, err
	}

	if err := setNode([]string{"metadata", "namespace"}, r.Namespace); err != nil {
		return nil, err
	}

	return rn, nil
}

// ObjectRunID returns the value of the LabelRunID annotation on the given object.
func ObjectRunID(u *unstructured.Unstructured) string {
	for key, val := range u.GetAnnotations() {
		if key == LabelRunID {
			return val
		}
	}

	return ""
}

// yamlKindStr stringifies the yaml.Kind since kyaml doesn't do that for us.
// nolint:unused,deadcode
func yamlKindStr(k yaml.Kind) string {
	switch k {
	case yaml.DocumentNode:
		return "Document"
	case yaml.SequenceNode:
		return "Sequence"
	case yaml.MappingNode:
		return "Mapping"
	case yaml.ScalarNode:
		return "Scalar"
	case yaml.AliasNode:
		return "Alias"
	default:
		return "huh?"
	}
}
