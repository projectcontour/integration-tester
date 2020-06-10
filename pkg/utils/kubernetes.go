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

package utils

import (
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/utils/pointer"
)

// ImmediateDeletionOptions returns metav1.DeleteOptions specifying
// that the caller requires immediate foreground deletion semantics.
func ImmediateDeletionOptions(propagation metav1.DeletionPropagation) *metav1.DeleteOptions {
	return &metav1.DeleteOptions{
		GracePeriodSeconds: pointer.Int64Ptr(0),
		PropagationPolicy:  &propagation,
	}
}

// NamespaceOrDefault returns the namespace from u, or "default" if u
// has no namespace field.
func NamespaceOrDefault(u *unstructured.Unstructured) string {
	if ns := u.GetNamespace(); ns != "" {
		return ns
	}

	return metav1.NamespaceDefault
}

// NewSelectorFromObject creates a selector to match all the labels in u.
func NewSelectorFromObject(u *unstructured.Unstructured) labels.Selector {
	return labels.SelectorFromSet(labels.Set(u.GetLabels()))
}

// SplitObjectName splits a string into namespace and name.
func SplitObjectName(fullName string) (string, string) {
	parts := strings.SplitN(fullName, "/", 2)
	switch len(parts) {
	case 1:
		return metav1.NamespaceDefault, parts[0]
	case 2:
		return parts[0], parts[1]
	default:
		panic(fmt.Sprintf("failed to split %q", fullName))
	}
}
