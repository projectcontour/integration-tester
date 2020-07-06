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

package test

import (
	"fmt"
	"strings"

	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/result"

	"sigs.k8s.io/yaml"
)

// TapWriter writes test records in TAP format.
// See https://testanything.org/tap-version-13-specification.html
type TapWriter struct {
	docCount  int
	stepCount int

	stepErrors []result.Result
	stepSkips  []result.Result
}

var _ Recorder = &TapWriter{}

// indentf prints a (possibly multi-line) message, prefixed by the indent.
// nolint(unparam)
func indentf(indent string, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	for _, line := range strings.Split(msg, "\n") {
		fmt.Printf("%s%s\n", indent, line)
	}
}

// ShouldContinue ...
func (t *TapWriter) ShouldContinue() bool {
	return true
}

// Failed ...
func (t *TapWriter) Failed() bool {
	return false
}

// NewDocument ...
func (t *TapWriter) NewDocument(desc string) Closer {
	// It's not obvious how TAP separates test runs into suites
	// (maybe it doesn't?). Let's stuff a newline in there so at
	// least it's visually distinguished.
	if t.docCount == 0 {
		fmt.Printf("TAP version 13\n")
	} else {
		fmt.Printf("\nTAP version 13\n")
	}

	t.docCount++
	t.stepCount = 0

	return CloserFunc(func() {
		// NOTE, it's a closed interval.
		fmt.Printf("1..%d\n", t.stepCount)
	})
}

// NewStep ...
func (t *TapWriter) NewStep(desc string) Closer {
	stepNum := t.stepCount + 1
	t.stepCount++

	return CloserFunc(func() {
		switch {
		case len(t.stepErrors) > 0:
			fmt.Printf("not ok %d - %s\n", stepNum, desc)
		case len(t.stepSkips) > 0:
			fmt.Printf("ok %d - %s # skip\n", stepNum, desc)
		default:
			fmt.Printf("ok %d - %s\n", stepNum, desc)
		}

		if len(t.stepErrors) > 0 {
			indent := "  "
			indentf(indent, "---")
			indentf(indent, string(must.Bytes(yaml.Marshal(t.stepErrors))))
			indentf(indent, "...")
		}

		t.stepErrors = nil
	})
}

// Update ...
func (t *TapWriter) Update(results ...result.Result) {
	for _, r := range results {
		switch r.Severity {
		case result.SeverityNone:
			indentf("# ", r.Message)
		case result.SeveritySkip:
			indentf(fmt.Sprintf("# %s - ", string(r.Severity)), r.Message)
			t.stepSkips = append(t.stepSkips, r)
		default:
			indentf(fmt.Sprintf("# %s - ", string(r.Severity)), r.Message)
			t.stepErrors = append(t.stepErrors, r)
		}
	}
}
