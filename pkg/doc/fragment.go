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

package doc

import (
	"bytes"
	"fmt"
	"io"

	"github.com/projectcontour/integration-tester/pkg/utils"

	"github.com/open-policy-agent/opa/ast"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/yaml"
)

const (
	// FragmentTypeUnknown indicates this Fragment is unknown
	// and needs to be decoded.
	FragmentTypeUnknown = iota
	// FragmentTypeInvalid indicates that this Fragment could not be parsed
	// or contains syntax errors.
	FragmentTypeInvalid
	// FragmentTypeObject indicates this Fragment contains a Kubernetes Object.
	FragmentTypeObject
	// FragmentTypeModule indicates this Fragment contains a Rego module.
	FragmentTypeModule
)

var _ error = &InvalidFragmentErr{}

// InvalidFragmentErr is an error value returned to indicate to the
// caller what type of fragment was found to be invalid.
type InvalidFragmentErr struct {
	// Type is the fragment type that was expected at the point
	// the error happened.
	Type FragmentType
}

func (e *InvalidFragmentErr) Error() string {
	return fmt.Sprintf("invalid %s fragment", e.Type)
}

// FragmentType is the parsed content type for the Fragment.
type FragmentType int

func (t FragmentType) String() string {
	switch t {
	case FragmentTypeObject:
		return "Kubernetes"
	case FragmentTypeModule:
		return "Rego"
	case FragmentTypeInvalid:
		return "invalid"
	default:
		return "unknown"
	}
}

// Location tracks the lines that bound a Fragment within some larger Document.
type Location struct {
	// Start is the line number this location starts on.
	Start int

	// End is the line number this location ends on.
	End int
}

func (l Location) String() string {
	return fmt.Sprintf("%d-%d", l.Start, l.End)
}

// Fragment is a parseable portion of a Document.
type Fragment struct {
	Bytes    []byte
	Type     FragmentType
	Location Location

	object *unstructured.Unstructured
	module *ast.Module
}

// Object returns the Kubernetes object if there is one.
func (f *Fragment) Object() *unstructured.Unstructured {
	switch f.Type {
	case FragmentTypeObject:
		return f.object
	default:
		return nil
	}
}

// Rego returns the Rego module if there is one.
func (f *Fragment) Rego() *ast.Module {
	switch f.Type {
	case FragmentTypeModule:
		return f.module
	default:
		return nil
	}
}

func hasKindVersion(u *unstructured.Unstructured) bool {
	k := u.GetObjectKind().GroupVersionKind()
	return len(k.Version) > 0 && len(k.Kind) > 0
}

func decodeYAMLOrJSON(data []byte) (*unstructured.Unstructured, error) {
	buffer := bytes.NewReader(data)
	decoder := yaml.NewYAMLOrJSONDecoder(buffer, buffer.Len())

	into := map[string]interface{}{}
	if err := decoder.Decode(&into); err != nil {
		return nil, err
	}

	return &unstructured.Unstructured{Object: into}, nil
}

func decodeModule(data []byte) (*ast.Module, error) {
	m, err := utils.ParseCheckFragment(string(data))
	if err != nil {
		return nil, err
	}

	// ParseModule can return nil with no error (empty module).
	if m == nil {
		return nil, io.EOF
	}

	return m, nil
}

// IsDecoded returns whether this fragment has been decoded to a known fragment type.
func (f *Fragment) IsDecoded() bool {
	switch f.Type {
	case FragmentTypeInvalid, FragmentTypeUnknown:
		return false
	default:
		return true
	}
}

// Decode attempts to parse the Fragment.
func (f *Fragment) Decode() (FragmentType, error) {
	if u, err := decodeYAMLOrJSON(f.Bytes); err == nil {
		// It's only a valid object if it has a version & kind.
		if hasKindVersion(u) {
			f.Type = FragmentTypeObject
			f.object = u
			return f.Type, nil
		}

		return FragmentTypeInvalid,
			utils.ChainErrors(
				&InvalidFragmentErr{Type: FragmentTypeObject},
				fmt.Errorf("YAML fragment is not a Kubernetes object"),
			)
	}

	// At this point, we don't strictly know that this fragment
	// should decode to Rego. However, if we don't assume that,
	// then we can't know whether to propagate Rego syntax errors.
	// Since we do want to propagate errors so that users can debug
	// scripts, we have to assume this is meant to be Rego.

	m, err := decodeModule(f.Bytes)
	if err != nil {
		return FragmentTypeInvalid,
			utils.ChainErrors(
				&InvalidFragmentErr{Type: FragmentTypeModule}, err,
			)
	}

	// Rego will parse raw JSON and YAML, but in that
	// case there won't be a any rules in the module.
	if len(m.Rules) == 0 {
		return FragmentTypeUnknown, nil
	}

	f.Type = FragmentTypeModule
	f.module = m
	return f.Type, nil
}

// NewRegoFragment decodes the given data and returns a new Fragment
// of type FragmentTypeModule.
func NewRegoFragment(data []byte) (*Fragment, error) {
	frag := Fragment{Bytes: data}

	fragType, err := frag.Decode()
	if err != nil {
		return nil, fmt.Errorf("%s: %s", err, utils.AsRegoCompilationErr(err))
	}

	if fragType != FragmentTypeModule {
		return nil, fmt.Errorf("unexpected fragment type %q", fragType)
	}

	return &frag, nil
}
