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
	"errors"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestErrChainUnwrap(t *testing.T) {
	e := ChainErrors(
		errors.New("one"),
		errors.New("two"),
		errors.New("three"),
	)

	assert.Equal(t, e.Error(), "one")

	e = errors.Unwrap(e)
	assert.Equal(t, e.Error(), "two")
	assert.NotNil(t, e)

	e = errors.Unwrap(e)
	assert.Equal(t, e.Error(), "three")
	assert.NotNil(t, e)

	e = errors.Unwrap(e)
	assert.Nil(t, e)
}

func TestErrChainAs(t *testing.T) {
	e := ChainErrors(
		errors.New("one"),
		os.ErrExist,
		&os.PathError{
			Op:   "test",
			Path: "/test/path",
			Err:  errors.New("tested error"),
		},
	)

	var pathError *os.PathError
	assert.True(t, errors.As(e, &pathError))

	var linkError *os.LinkError
	assert.False(t, errors.As(e, &linkError))
}

func TestErrChainIs(t *testing.T) {
	is := fmt.Errorf("this error is")
	isnot := fmt.Errorf("this error is not")

	e := ChainErrors(
		errors.New("one"),
		os.ErrExist,
		is,
	)

	assert.True(t, errors.Is(e, is))
	assert.True(t, errors.Is(e, os.ErrExist))
	assert.False(t, errors.Is(e, isnot))
}
