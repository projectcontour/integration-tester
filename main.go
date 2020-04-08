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

package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/projectcontour/integration-tester/cmd"
	"github.com/projectcontour/integration-tester/pkg/version"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	rand.Seed(time.Now().UnixNano())

	if err := cmd.NewRootCommand().Execute(); err != nil {
		if msg := err.Error(); msg != "" {
			fmt.Fprintf(os.Stderr, "%s: %s\n", version.Progname, msg)
		}

		var exit *cmd.ExitError
		if errors.As(err, &exit) {
			os.Exit(int(exit.Code))
		}

		os.Exit(int(cmd.EX_FAIL))
	}
}
