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

package utils

import (
	"math/rand"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRandomStringN(t *testing.T) {
	rand.Seed(1)

	assert.Equal(t, "", RandomStringN(-1))
	assert.Equal(t, "", RandomStringN(0))
	assert.Equal(t, "oJnNPG", RandomStringN(6))
	assert.Equal(t, "siuzytMOJPa", RandomStringN(11))
}

func TestJoinLines(t *testing.T) {
	lines := []string{"one", "two", "three"}
	assert.Equal(t, strings.Join(lines, "\n"), JoinLines(lines...))
}
