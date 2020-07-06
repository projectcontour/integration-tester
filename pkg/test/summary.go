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
	"io"
	"text/tabwriter"

	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/result"
)

type docSummary struct {
	doc    string
	status result.Severity
}

// SummaryWriter collects a summary of the final test results.
type SummaryWriter struct {
	currentDoc *docSummary
	docResults []docSummary
}

var _ Recorder = &SummaryWriter{}

// ShouldContinue ...
func (s *SummaryWriter) ShouldContinue() bool {
	return true
}

// Failed ...
func (s *SummaryWriter) Failed() bool {
	return false
}

// NewDocument ...
func (s *SummaryWriter) NewDocument(desc string) Closer {
	s.currentDoc = &docSummary{doc: desc, status: result.SeverityNone}
	return CloserFunc(func() {
		s.docResults = append(s.docResults, *s.currentDoc)
		s.currentDoc = nil
	})
}

// NewStep ...
func (s *SummaryWriter) NewStep(desc string) Closer {
	return CloserFunc(nil)
}

// Update ...
func (s *SummaryWriter) Update(results ...result.Result) {
	for _, r := range results {
		switch r.Severity {
		case result.SeverityFatal,
			result.SeverityError,
			result.SeveritySkip:
			s.currentDoc.status = r.Severity
		}
	}
}

// Summarize write a summary of the test results to out.
func (s *SummaryWriter) Summarize(out io.Writer) {
	summaryNames := map[result.Severity]string{
		result.SeverityError: "FAILED",
		result.SeverityFatal: "FAILED",
		result.SeverityNone:  "PASSED",
		result.SeveritySkip:  "SKIPPED",
	}

	tab := tabwriter.NewWriter(out, 0, 4, 4, ' ', 0)

	fmt.Fprintf(tab, "\n")

	for _, r := range s.docResults {
		fmt.Fprintf(tab, "%s\t%s\n", r.doc, summaryNames[r.status])
	}

	must.Must(tab.Flush())
}
