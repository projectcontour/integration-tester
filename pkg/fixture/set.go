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
	"sync"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// FixtureSet is a collection of fixture objects.
// nolint(golint)
type FixtureSet interface {
	Insert(Key, Fixture)
	Match(u *unstructured.Unstructured) Fixture
}

// Key is the indexing fixture set key.
type Key struct {
	apiVersion string
	kind       string
	name       string
	namespace  string
}

// KeyFor returns the key for indexing the given object.
func KeyFor(u *unstructured.Unstructured) Key {
	return Key{
		apiVersion: u.GetAPIVersion(),
		kind:       u.GetKind(),
		name:       u.GetName(),
		namespace:  u.GetNamespace(),
	}
}

type defaultFixtureSet struct {
	lock     sync.Mutex
	fixtures map[Key]Fixture
}

var _ FixtureSet = &defaultFixtureSet{}

// Insert a fixture with the given key.
func (s *defaultFixtureSet) Insert(k Key, f Fixture) {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.fixtures[k] = f
}

// Match the given object to an existing Fixture.
func (s *defaultFixtureSet) Match(u *unstructured.Unstructured) Fixture {
	s.lock.Lock()
	defer s.lock.Unlock()

	// Assume that the caller will not modify the result.
	return s.fixtures[KeyFor(u)]
}

// Set is the default FixtureSet.
var Set = &defaultFixtureSet{
	fixtures: map[Key]Fixture{},
}
