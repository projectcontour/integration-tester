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

package result

import (
	"fmt"
	"time"
)

// Severity indicates the seriousness of a Result.
type Severity string

// SeverityNone ...
const SeverityNone Severity = "None"

// SeverityError ...
const SeverityError Severity = "Error"

// SeverityFatal ...
const SeverityFatal Severity = "Fatal"

// SeveritySkip ...
const SeveritySkip Severity = "Skip"

// Result ...
type Result struct {
	Severity  Severity
	Message   string
	Timestamp time.Time
}

// IsTerminal returns true if this result should end the test.
func (c Result) IsTerminal() bool {
	switch c.Severity {
	case SeverityFatal, SeveritySkip:
		return true
	default:
		return false
	}
}

// IsFailed returns true if this result is a test failure.
func (c Result) IsFailed() bool {
	switch c.Severity {
	case SeverityFatal, SeverityError:
		return true
	default:
		return false
	}
}

func resultFrom(s Severity, format string, args ...interface{}) Result {
	return Result{
		Severity:  s,
		Message:   fmt.Sprintf(format, args...),
		Timestamp: time.Now(),
	}
}

// Infof formats a SeverityNone result.
func Infof(format string, args ...interface{}) Result {
	return resultFrom(SeverityNone, format, args...)
}

// Errorf formats a SeverityError result.
func Errorf(format string, args ...interface{}) Result {
	return resultFrom(SeverityError, format, args...)
}

// Fatalf formats a SeverityFatal result.
func Fatalf(format string, args ...interface{}) Result {
	return resultFrom(SeverityFatal, format, args...)
}

// Skipf formats a SeveritySkip result.
func Skipf(format string, args ...interface{}) Result {
	return resultFrom(SeveritySkip, format, args...)
}

// Contains returns true if the results slice has an element with
// the wanted Severity.
func Contains(results []Result, wanted Severity) bool {
	for _, r := range results {
		if r.Severity == wanted {
			return true
		}
	}

	return false
}
