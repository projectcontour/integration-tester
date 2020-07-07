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
	"time"

	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/result"
)

// Document records the execution of a test document.
type Document struct {
	Description string
	Properties  map[string]interface{}
	Steps       []*Step
}

// EachResult walks the test document and applies the function to
// each error.
func (d *Document) EachResult(f func(*Step, *result.Result)) {
	for _, s := range d.Steps {
		for _, r := range s.Results {
			r := r
			f(s, &r)
		}
	}
}

// Step describes a stage in a test document that can generate onr
// or more related errors.
type Step struct {
	Description string
	Start       time.Time
	End         time.Time
	Results     []result.Result
	Diagnostics map[string]interface{}
}

// Closer is an interface that closes an implicit test tracking entity.
type Closer interface {
	Close()
}

// CloserFunc is a Closer adaptor. This adaptor can be used with nil function pointers.
type CloserFunc func()

// Close implements Closer.
func (c CloserFunc) Close() {
	if c != nil {
		c()
	}
}

// Recorder is an object that records structured test information.
type Recorder interface {
	// ShouldContinue returns whether a test harness should
	// continue to run tests. Typically, this will return false
	// if a fatal test error has been reported.
	ShouldContinue() bool

	// Failed returns true if any errors have been reported.
	Failed() bool

	// NewDocument created a new test document that can be
	// closed by calling the returned Closer.
	NewDocument(desc string) Closer

	// NewDocument created a new test document that can be
	// closed by calling the returned Closer.
	NewStep(desc string) Closer

	Update(...result.Result)
}

type defaultRecorder struct {
	docs []*Document

	currentDoc  *Document
	currentStep *Step
}

// DefaultRecorder ...
var DefaultRecorder Recorder = &defaultRecorder{}

// ShouldContinue returns false if any fatal errors have been recorded.
func (r *defaultRecorder) ShouldContinue() bool {
	terminal := false

	// Make the check context-dependent. If we are in the middle
	// of a doc, this asks whether we should keep going on the
	// doc, otherwise it asks whether we should keep going at all.
	which := r.docs
	if r.currentDoc != nil {
		which = []*Document{r.currentDoc}
	}

	for _, d := range which {
		d.EachResult(func(s *Step, r *result.Result) {
			if r.IsTerminal() {
				terminal = true
			}
		})
	}

	return !terminal
}

// Failed returns true if any errors have been recorded.
func (r *defaultRecorder) Failed() bool {
	failed := false

	for _, d := range r.docs {
		d.EachResult(func(s *Step, r *result.Result) {
			if r.IsFailed() {
				failed = true
			}
		})
	}

	return failed
}

// NewDocument creates a new Document and makes it current.
func (r *defaultRecorder) NewDocument(desc string) Closer {
	must.Check(r.currentStep == nil,
		fmt.Errorf("can't create a new doc with an open step"))

	doc := &Document{}

	r.currentDoc = doc
	r.docs = append(r.docs, doc)

	return CloserFunc(func() {
		must.Check(r.currentDoc == doc,
			fmt.Errorf("overlapping docs"))
		must.Check(r.currentStep == nil,
			fmt.Errorf("closing doc with open step"))

		r.currentDoc = nil
	})
}

// NewStep creates a new Step within the current Document and makes
// that the current Step.
func (r *defaultRecorder) NewStep(desc string) Closer {
	must.Check(r.currentDoc != nil,
		fmt.Errorf("no open document"))

	step := &Step{
		Description: desc,
		Start:       time.Now(),
	}

	r.currentStep = step
	r.currentDoc.Steps = append(r.currentDoc.Steps, step)

	return CloserFunc(func() {
		must.Check(r.currentStep == step,
			fmt.Errorf("overlapping steps"))

		step.End = time.Now()

		r.currentStep = nil
	})
}

func (r *defaultRecorder) Update(res ...result.Result) {
	must.Check(r.currentStep != nil, fmt.Errorf("no open step"))
	r.currentStep.Results = append(r.currentStep.Results, res...)
}
