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

package cmd

import (
	"fmt"
	"strings"
	"time"

	"github.com/projectcontour/integration-tester/pkg/doc"
	"github.com/projectcontour/integration-tester/pkg/driver"
	"github.com/projectcontour/integration-tester/pkg/fixture"
	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/result"
	"github.com/projectcontour/integration-tester/pkg/test"
	"github.com/projectcontour/integration-tester/pkg/utils"
	"github.com/projectcontour/integration-tester/pkg/version"

	"github.com/open-policy-agent/opa/ast"
	"github.com/spf13/cobra"
)

// NewRunCommand returns a command ro run a test case.
func NewRunCommand() *cobra.Command {
	run := &cobra.Command{
		Use:   "run [FLAGS ...] FILE [FILE ...]",
		Short: "Run a set of test documents",
		Long: `Execute a set of test documents given as arguments.

Test documents are ordered fragments of YAML object and Rego checks,
separated by the YAML document separator, '---'. The fragments in the
test document are executed sequentially.

If a Kubernetes object specifies a target namespace in its metadata,
integration-tester will implicitly create and manage that namespace.
This reduces test verbosity be not requiring namespace YAML fragments.

When integration-tester creates Kubernetes objects, it uses the current
default Kubernetes client context. Each Kubernetes object it creates
is labeled with the 'app.kubernetes.io/managed-by=integration-tester'
label. Objects are also annotated with a unique test run ID under the
key 'integration-tester/run-id'

integration-tester will delete the target Kubernetes object if the special
'$apply' key has the value 'delete'. If the target object has a name,
integration-tester will delete that object. Otherwise, integration-tester
will attempt to select an object to delete by matching the run ID and
any specified labels.

Unless the '--preserve' flag is specified, integration-tester will
automatically delete all the Kubernetes objects it created at the
end of each test.

Since both Kubernetes and the services in a cluster are eventually
consistent, checks are executed repeatedly until they succeed or
until the timeout given by the '--check-timeout' flag expires.

The '--param' flag can be provided multiple times to add an element
to the Rego data store. The argument to this flag is a "key=value"
pair. The value is stored as 'data.test.params.key'.

integration-tester will automatically watch resource types that are
created in a test document and publish them into Rego checks in the
'data.resources' tree. If a test needs to inspect more resources, the
'--watch' flag can be provided multiple times to specify additional
resource types to monitor and publish.

The test results output format can be changed by the '--format' flag.
The default format is 'tree', which is a custom hierarchical format
suitable for terminals. The "tap" format emits TAP (Test Anything
Protocol) results.
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return ExitErrorf(EX_USAGE, "no test file(s)")
			}

			return runCmd(cmd, args)
		},
	}

	run.Flags().String("trace", "", "Set execution tracing flags")
	run.Flags().Bool("preserve", false, "Don't automatically delete Kubernetes objects")
	run.Flags().Bool("dry-run", false, "Don't actually create Kubernetes objects")
	run.Flags().Duration("check-timeout", time.Second*30, "Timeout for evaluating check steps")
	run.Flags().StringArray("param", []string{}, "Additional Rego parameter(s) in key=value format")
	run.Flags().StringSlice("watch", []string{}, "Additional Kubernetes resources to monitor")
	run.Flags().StringSlice("fixtures", []string{}, "Additional Kubernetes resource fixtures")
	run.Flags().StringSlice("policies", []string{}, "Additional Rego policy packages")
	run.Flags().String("format", "tree", "Test results output format")

	return CommandWithDefaults(run)
}

func runCmd(cmd *cobra.Command, args []string) error {
	traceFlags := strings.Split(must.String(cmd.Flags().GetString("trace")), ",")

	if err := loadFixtures(
		must.StringSlice(cmd.Flags().GetStringSlice("fixtures"))); err != nil {
		return ExitError{Code: EX_NOINPUT, Err: err}
	}

	paramOpts, err := validateParams(
		must.StringSlice(cmd.Flags().GetStringArray("param")))
	if err != nil {
		return err
	}

	kube, err := driver.NewKubeClient()
	if err != nil {
		return fmt.Errorf("failed to initialize Kubernetes context: %s", err)
	}

	var recorder test.Recorder

	switch must.String(cmd.Flags().GetString("format")) {
	case "tree":
		recorder = test.StackRecorders(&test.TreeWriter{}, test.DefaultRecorder)
	case "tap":
		recorder = test.StackRecorders(&test.TapWriter{}, test.DefaultRecorder)
	default:
		return ExitErrorf(EX_USAGE, "invalid test output format %q",
			must.String(cmd.Flags().GetString("format")))
	}

	opts := []test.RunOpt{
		test.KubeClientOpt(kube),
		test.RecorderOpt(recorder),
		test.CheckTimeoutOpt(must.Duration(cmd.Flags().GetDuration("check-timeout"))),
	}

	opts = append(opts, paramOpts...)

	if must.Bool(cmd.Flags().GetBool("preserve")) {
		opts = append(opts, test.PreserveObjectsOpt())
	}

	if must.Bool(cmd.Flags().GetBool("dry-run")) {
		opts = append(opts, test.DryRunOpt())
	}

	if utils.ContainsString(traceFlags, "rego") {
		opts = append(opts, test.TraceRegoOpt())
	}

	if names := must.StringSlice(cmd.Flags().GetStringSlice("watch")); len(names) > 0 {
		for _, n := range names {
			gvrs, err := kube.ResourcesForName(n)
			if err != nil {
				return err
			}

			for _, gvr := range gvrs {
				opts = append(opts, test.WatchResourceOpt(gvr))
			}
		}
	}

	if policies := must.StringSlice(cmd.Flags().GetStringSlice("policies")); len(policies) > 0 {
		modules, err := loadPolicies(policies)
		if err != nil {
			return ExitError{
				Code: EX_DATAERR,
				Err:  err,
			}
		}

		for _, m := range modules {
			opts = append(opts, test.RegoModuleOpt(m))
		}
	}

	// TODO(jpeach): set user agent from program version.
	kube.SetUserAgent(fmt.Sprintf("%s/%s", version.Progname, version.Version))

	for _, path := range args {
		docCloser := recorder.NewDocument(path)
		testDoc := validateDocument(path, recorder)

		if recorder.ShouldContinue() {
			if err := test.Run(testDoc, opts...); err != nil {
				return fmt.Errorf("failed to run tests: %s", err)
			}
		}

		docCloser.Close()
	}

	if recorder.Failed() {
		return ExitError{Code: EX_FAIL}
	}

	return nil
}

func loadPolicies(paths []string) (map[string]*ast.Module, error) {
	modules := map[string]*ast.Module{}
	loadPath := func(filePath string) error {
		m, err := utils.ParseModuleFile(filePath)
		if err != nil {
			return err
		}

		modules[filePath] = m
		return nil
	}

	for _, p := range paths {
		if err := utils.WalkFiles(p, loadPath); err != nil {
			return nil, err
		}
	}

	// Verify that the policies compile. We compile them all at
	// the end so that the compiler can resolve any dependencies.
	compiler := ast.NewCompiler()
	if compiler.Compile(modules); compiler.Failed() {
		return modules, compiler.Errors
	}

	return modules, nil
}

func loadFixtures(paths []string) error {
	loadPath := func(filePath string) error {
		if err := fixture.AddFromFile(filePath); err != nil {
			return fmt.Errorf("failed to parse %q`: %w", filePath, err)
		}

		return nil
	}

	for _, p := range paths {
		if err := utils.WalkFiles(p, loadPath); err != nil {
			return err
		}
	}

	return nil
}

func validateParams(params []string) ([]test.RunOpt, error) {
	opts := []test.RunOpt{}

	for _, p := range params {
		parts := strings.SplitN(p, "=", 2)
		if len(parts) != 2 {
			return nil, fmt.Errorf("missing value for parameter %q", parts[0])
		}

		opts = append(opts, test.RegoParamOpt(parts[0], parts[1]))
	}

	return opts, nil
}

func validateDocument(path string, r test.Recorder) *doc.Document {
	stepCloser := r.NewStep(fmt.Sprintf("validating document %q", path))
	defer stepCloser.Close()

	r.Update(result.Infof("reading document from %s", path))

	testDoc, err := doc.ReadFile(path)
	if err != nil {
		r.Update(result.Fatalf("%s", err.Error()))
		return nil
	}

	r.Update(result.Infof(
		"decoding document with %d parts from %s", len(testDoc.Parts), path))

	// Before executing anything, verify that we can decode all the
	// fragments and raise any syntax errors.
	for i := range testDoc.Parts {
		part := &testDoc.Parts[i]
		fragType, err := part.Decode()
		switch err {
		case nil:
			r.Update(result.Infof("decoded part %d as %s (lines %s)", i, fragType, part.Location))
		default:
			if regoErr := utils.AsRegoCompilationErr(err); regoErr != nil {
				r.Update(result.Fatalf("%s", regoErr.Error()))
			} else {
				r.Update(result.Fatalf("%s", err.Error()))
			}
		}
	}

	return testDoc
}
