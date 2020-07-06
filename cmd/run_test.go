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

package cmd

import (
	"testing"

	"github.com/projectcontour/integration-tester/pkg/test"
	"github.com/stretchr/testify/assert"
)

func TestParamValidation(t *testing.T) {
	opts, err := validateParams([]string{})
	assert.NoError(t, err)
	assert.Equal(t, []test.RunOpt{}, opts)

	opts, err = validateParams([]string{"foo"})
	assert.Error(t, err)
	assert.Equal(t, []test.RunOpt(nil), opts)

	opts, err = validateParams([]string{"foo=bar=baz=fizz"})
	assert.NoError(t, err)
	assert.Equal(t, 1, len(opts))

	opts, err = validateParams([]string{"foo=bar", "foo=bar"})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(opts))
}
