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
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewNamespace(t *testing.T) {
	u := NewNamespace("foo")

	assert.NotNil(t, u)
	assert.Equal(t, u.GetNamespace(), "")
	assert.Equal(t, u.GetName(), "foo")
	assert.Equal(t, u.GetKind(), "Namespace")
	assert.Equal(t, u.GetAPIVersion(), "v1")
}
