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

package utils

import (
	"os"
	"path"
	"path/filepath"
	"strings"
)

// IsDirPath returns true if path refers to a directory.
func IsDirPath(path string) bool {
	if info, err := os.Stat(path); err == nil {
		return info.IsDir()
	}

	return false
}

// WalkFiles is a wrapper around filepath.Walk that accepts a path
// that may be either a file or a directory. In either case, it recurses
// the path and applied walkFunc to all files that it finds. Hidden
// files (i.e. dotfiles) are ignored.
func WalkFiles(walkPath string, walkFn func(string) error) error {
	if IsDirPath(walkPath) {
		return filepath.Walk(walkPath, func(filePath string, info os.FileInfo, err error) error {
			// If we already have an error, don't keep walking.
			if err != nil {
				return err
			}

			// Skip (hidden) dotfiles.
			if strings.HasPrefix(path.Base(filePath), ".") {
				return nil
			}

			// Nothing to do on directories.
			if info.IsDir() {
				return nil
			}

			return walkFn(filePath)
		})
	}

	return walkFn(walkPath)
}
