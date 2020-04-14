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

package driver

import (
	"strings"

	"github.com/projectcontour/integration-tester/pkg/result"

	"github.com/open-policy-agent/opa/ast"
)

type ruleInfo struct {
	name     string
	prefix   string
	severity result.Severity
}

var rules = []ruleInfo{
	// The following rules cause a tet failure if they are ever true.
	{name: "error", prefix: "error_", severity: result.SeverityError},
	{name: "fatal", prefix: "fatal_", severity: result.SeverityFatal},
	{name: "skip", prefix: "skip_", severity: result.SeveritySkip},
}

// matchRuleByName finds the ruleInfo that matches the given query
// name exactly, or by prefix.
func matchRuleByName(name string) *ruleInfo {
	for _, q := range rules {
		if name == q.name || strings.HasPrefix(name, q.prefix) {
			return &q
		}
	}

	return nil
}

// severityForRuleName returns the test severity for a given rule name.
func severityForRuleName(name string) result.Severity {
	if q := matchRuleByName(name); q != nil {
		return q.severity
	}

	return result.SeverityNone
}

// queryForRuleName returns a Rego query for the given rule name. This
// is currently a no-op, but is a placeholder for allowing non-identity
// queries against rules.
func queryForRuleName(name string) string {
	if q := matchRuleByName(name); q != nil {
		return name
	}

	return ""
}

// findAssertionRules searches the module for rules that match a
// test assertion severity.
func findAssertionRules(m *ast.Module) []string {
	// The rule names we match in a hash because the same rule
	// name can appear more than once in a policy document.
	found := map[string]struct{}{}

	for _, rule := range m.Rules {
		name := rule.Head.Name.String()

		if severityForRuleName(name) == result.SeverityNone {
			continue
		}

		found[name] = struct{}{}
	}

	// Flatten query names back into the slice.
	result := make([]string, 0, len(found))
	for k := range found {
		result = append(result, k)
	}

	return result
}
