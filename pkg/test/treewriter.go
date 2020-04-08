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

package test

import (
	"fmt"
	"strings"
	"time"

	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/result"
)

type leader string

const (
	// Fixed-width boxing characters.
	boxBranch   = "├─"
	boxVertical = "│ "
	boxLeft     = "└─"

	// tabPrintf leaders are boxing characters with a bit of
	// fixed breathing space.
	branchLeader leader = boxBranch + " "
	elbowLeader  leader = boxLeft + " "
	emptyLeader  leader = ""
)

func formatIndent(n int) string {
	b := strings.Builder{}
	b.Grow(n * len(boxVertical))

	for i := 0; i < n; i++ {
		must.Int(b.WriteString(boxVertical))
	}

	return b.String()
}

func formatFailCounters(fails map[result.Severity]int) string {
	b := strings.Builder{}

	pluralize := func(s result.Severity, n int) string {
		switch n {
		case 1:
			return map[result.Severity]string{
				result.SeverityError: "error",
				result.SeverityFatal: "error",
			}[s]
		default:
			return map[result.Severity]string{
				result.SeverityError: "errors",
				result.SeverityFatal: "errors",
			}[s]
		}
	}

	if n := fails[result.SeverityError] + fails[result.SeverityFatal]; n > 0 {
		must.Int(b.WriteString(
			fmt.Sprintf("%d %s", n, pluralize(result.SeverityError, n))))
	}

	return b.String()
}

// TreeWriter is a Recorder that write test results to a standard
// output in a tree notation.
type TreeWriter struct {
	indent    int
	docCount  int
	stepCount int

	stepErrors map[result.Severity]int
	allErrors  map[result.Severity]int
}

var _ Recorder = &TreeWriter{}

func tabPrintf(indent int, leader leader, format string, args ...interface{}) {
	timestamp := time.Now().Format("15:04:05.0000")
	msg := fmt.Sprintf(format, args...)
	lines := strings.Split(msg, "\n")

	for n, line := range lines {
		// Format the leader only on the first output line,
		// replacing it with an extra indent on subsequent
		// lines. This makes branchLeader entries look better,
		// but will horrendously munge elbowLeader ones (the
		// logic needs to be reversed).
		if n == 0 {
			fmt.Printf("%s\t%s%s%s\n",
				timestamp, formatIndent(indent), leader, line)
		} else {
			fmt.Printf("%s\t%s %s\n",
				timestamp, formatIndent(indent+1), line)
		}
	}
}

// ShouldContinue ...
func (t *TreeWriter) ShouldContinue() bool {
	return true
}

// Failed ...
func (t *TreeWriter) Failed() bool {
	return false
}

// NewDocument ...
func (t *TreeWriter) NewDocument(desc string) Closer {
	if t.docCount > 0 {
		fmt.Printf("\n")
	}

	tabPrintf(t.indent, emptyLeader, "Running: %s", desc)

	t.docCount++
	t.stepCount = 0
	t.allErrors = map[result.Severity]int{}

	return CloserFunc(func() {
		switch {
		case t.allErrors[result.SeveritySkip] > 0:
			tabPrintf(t.indent, elbowLeader, "Skipped after %d steps", t.stepCount)
		case (t.allErrors[result.SeverityFatal] + t.allErrors[result.SeverityError]) > 0:
			tabPrintf(t.indent, elbowLeader,
				"Failed with %s ", formatFailCounters(t.allErrors))
		default:
			tabPrintf(t.indent, elbowLeader, "Pass with %d steps OK", t.stepCount)
		}
	})
}

// NewStep ...
func (t *TreeWriter) NewStep(desc string) Closer {
	tabPrintf(t.indent, branchLeader, "Step %d: %s", t.stepCount, desc)

	t.indent++
	t.stepCount++
	t.stepErrors = map[result.Severity]int{}

	return CloserFunc(func() {
		switch {
		case t.stepErrors[result.SeveritySkip] > 0:
			tabPrintf(t.indent, elbowLeader, "Skipped")
		case (t.stepErrors[result.SeverityFatal] + t.stepErrors[result.SeverityError]) > 0:
			tabPrintf(t.indent, elbowLeader,
				"Failed with %s ", formatFailCounters(t.stepErrors))
		default:
			tabPrintf(t.indent, elbowLeader, "Pass")
		}

		t.indent--
		for k, v := range t.stepErrors {
			t.allErrors[k] = t.allErrors[k] + v
		}
	})
}

// Update ...
func (t *TreeWriter) Update(results ...result.Result) {
	for _, r := range results {
		switch r.Severity {
		case result.SeverityNone:
			tabPrintf(t.indent, branchLeader, "%s", r.Message)
		default:
			t.stepErrors[r.Severity]++
			tabPrintf(t.indent, branchLeader, "%s: %s", strings.ToUpper(string(r.Severity)), r.Message)
		}
	}
}
