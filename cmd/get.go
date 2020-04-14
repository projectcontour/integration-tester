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
	"log"
	"strings"

	"github.com/projectcontour/integration-tester/pkg/driver"
	"github.com/projectcontour/integration-tester/pkg/filter"
	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/version"

	"github.com/gosuri/uitable"
	"github.com/spf13/cobra"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/duration"
)

// NewGetCommand returns a new "get" command tree.
func NewGetCommand() *cobra.Command {
	get := &cobra.Command{
		Use:          "get",
		Short:        "Gets one of [objects, tests]",
		Long:         "Gets one of [objects, tests]",
		SilenceUsage: true,
	}

	objects := &cobra.Command{
		Use:   "objects [FLAGS ...]",
		Short: "Gets one Kubernetes objects",
		Long: fmt.Sprintf(
			`Gets Kubernetes objects managed by tests

This command lists Kubernetes API objects that are labeled as managed
by integration-tester. integration-tester labels objects created or
modified by test documents with the %s%s%s label.
`,
			"`", filter.LabelManagedBy, "`"),
		RunE: func(cmd *cobra.Command, args []string) error {
			kube, err := driver.NewKubeClient()
			if err != nil {
				return fmt.Errorf("failed to initialize Kubernetes context: %s", err)
			}

			results, err := kube.SelectObjectsByLabel(filter.LabelManagedBy, version.Progname)
			if err != nil {
				log.Printf("%s", err)
				return err
			}

			if len(results) == 0 {
				return nil
			}

			now := metav1.Now()
			table := uitable.New()
			table.AddRow("NAMESPACE", "NAME", "RUN ID", "AGE")

			for _, r := range results {
				gk := r.GetObjectKind().GroupVersionKind().GroupKind()
				name := fmt.Sprintf("%s/%s", strings.ToLower(gk.String()), r.GetName())
				age := now.Sub(r.GetCreationTimestamp().UTC())

				table.AddRow(
					r.GetNamespace(),
					name,
					must.String(kube.RunIDFor(r)),
					duration.HumanDuration(age),
				)
			}

			fmt.Println(table)
			return nil
		},
	}

	get.AddCommand(CommandWithDefaults(objects))
	return CommandWithDefaults(get)
}
