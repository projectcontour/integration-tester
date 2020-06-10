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
	"bufio"
	"bytes"
	"io"
	"os"
	"regexp"

	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/utils"
)

// Document is a collection of related Fragments.
type Document struct {
	Name  string
	Parts []Fragment
}

// ReadDocument reads a stream of Fragments that are separated by a
// YAML document separator (see https://yaml.org/spec/1.0/#id2561718).
// The contents of each Fragment is opaque and need not be YAML.
func ReadDocument(in io.Reader) (*Document, error) {
	startLine := 0
	currentLine := 0

	yamlSeparator := regexp.MustCompile("^---[\t\f\r ]*$")

	buf := bytes.Buffer{}
	doc := Document{}

	scanner := bufio.NewScanner(in)

	// Scan the input a line at a time.
	for scanner.Scan() {
		currentLine++
		if startLine == 0 {
			startLine = currentLine
		}

		// We just read another line, so replace the newline separator.
		if buf.Len() > 0 {
			must.Int(buf.WriteString("\n"))
		}

		if yamlSeparator.Match(scanner.Bytes()) {
			// Fragment must be at least one line long.
			// If we kept empty fragments, then we would
			// not be able to sel the line counts properly,
			// since YAML separators are not included.
			if buf.Len() > 0 {
				doc.Parts = append(doc.Parts, Fragment{
					Bytes: utils.CopyBytes(buf.Bytes()),
					Location: Location{
						Start: startLine,
						End:   currentLine - 1,
					},
				})
			}

			startLine = 0
			buf.Truncate(0)
			continue
		}

		must.Int(buf.Write(scanner.Bytes()))
	}

	// Append any data from the last separator up until EOF.
	if buf.Len() > 0 {
		doc.Parts = append(doc.Parts, Fragment{
			Bytes: utils.CopyBytes(buf.Bytes()),
			Location: Location{
				Start: startLine,
				End:   currentLine,
			},
		})
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return &doc, nil
}

// ReadFile reads a Document from the given file path.
func ReadFile(filePath string) (*Document, error) {
	fh, err := os.OpenFile(filePath, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}

	defer fh.Close() // nolint:gosec

	doc, err := ReadDocument(fh)
	if err != nil {
		return nil, err
	}

	doc.Name = filePath
	return doc, nil
}
