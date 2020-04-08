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

package builtin

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFileCaptureSimple(t *testing.T) {
	name := "capture_test.go"

	err := CaptureAsset(name)
	assert.NoError(t, err)
	assert.Contains(t, AssetNames(), name)

	_, err = Asset(name)
	assert.NoError(t, err)

	// This file should should show up in the root.
	names, err := AssetDir("")
	assert.NoError(t, err)
	assert.Contains(t, names, name)
}

func TestFileCapturePath(t *testing.T) {
	f := func() (*asset, error) {
		return nil, nil
	}

	assert.NoError(t, insertAssetAtPath("internal/test/one", f))
	assert.NoError(t, insertAssetAtPath("internal/test/two", f))

	assert.Contains(t, AssetNames(), "internal/test/one")
	assert.Contains(t, AssetNames(), "internal/test/two")

	names, err := AssetDir("internal/test")
	assert.NoError(t, err)
	assert.Contains(t, names, "one")
	assert.Contains(t, names, "two")

	// Can't insert across a file path.
	assert.Error(t, insertAssetAtPath("internal/test/two/three", f))
}

func TestFileCaptureDir(t *testing.T) {
	// Hacky, but need to delete global data added by path capture tests.
	delete(_bintree.Children, "capture_test.go")
	delete(_bindata, "capture_test.go")

	assert.NoError(t, CaptureAssets("."))
}
