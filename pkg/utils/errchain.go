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

package utils

import (
	"errors"
)

type chain struct {
	err  error
	next *chain
}

func (c *chain) Unwrap() error {
	if c.next == nil {
		return nil
	}

	return c.next
}

func (c *chain) Is(target error) bool {
	return errors.Is(c.err, target)
}

func (c *chain) As(target interface{}) bool {
	return errors.As(c.err, target)
}

func (c *chain) Error() string {
	return c.err.Error()
}

// ChainErrors takes the slice of errors and constructs a single chained
// error from is. The captures errors can be retrieved by inspecting the
// result with errors.As and errors.Is.
func ChainErrors(errs ...error) error {
	var head *chain
	var tail *chain

	for _, e := range errs {
		if tail == nil {
			head = &chain{err: e}
			tail = head
		} else {
			tail.next = &chain{err: e}
			tail = tail.next
		}
	}

	return head
}
