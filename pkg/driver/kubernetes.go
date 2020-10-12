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

package driver

import (
	"context"
	"errors"
	"log"

	"github.com/projectcontour/integration-tester/pkg/filter"
	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/utils"

	v1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// KubeClient collects various Kubernetes client interfaces.
type KubeClient struct {
	Config    *rest.Config // XXX(jpeach): remove this, it's only needed for init
	Client    *kubernetes.Clientset
	Dynamic   dynamic.Interface
	Discovery discovery.CachedDiscoveryInterface
}

// SetUserAgent sets the HTTP User-Agent on the Client.
func (k *KubeClient) SetUserAgent(ua string) {
	// XXX(jpeach): user agent is captured at create time, so keeping the config here doesn't help ...
	k.Config.UserAgent = ua
}

// NamespaceExists tests whether the given namespace is present.
func (k *KubeClient) NamespaceExists(nsName string) (bool, error) {
	_, err := k.Client.CoreV1().Namespaces().Get(context.Background(), nsName, metav1.GetOptions{})
	switch {
	case err == nil:
		return true, nil
	case apierrors.IsNotFound(err):
		return false, nil
	default:
		return true, err
	}
}

func (k *KubeClient) findAPIResourceForKind(kind schema.GroupVersionKind) (metav1.APIResource, error) {
	resources, err := k.Discovery.ServerResourcesForGroupVersion(
		schema.GroupVersion{Group: kind.Group, Version: kind.Version}.String())
	if err != nil {
		return metav1.APIResource{}, err
	}

	// The listed resources will have empty Group and Version
	// fields, which means that they are the same as that of the
	// list. Parse the list's GroupVersion to populate the result.
	gv := must.GroupVersion(schema.ParseGroupVersion(resources.GroupVersion))

	for _, r := range resources.APIResources {
		if r.Kind == kind.Kind {
			if r.Group == "" {
				r.Group = gv.Group
			}

			if r.Version == "" {
				r.Version = gv.Version
			}

			return r, nil
		}
	}

	return metav1.APIResource{}, errors.New("no match for kind")
}

// KindIsNamespaced returns whether the given kind can be created within a namespace.
func (k *KubeClient) KindIsNamespaced(kind schema.GroupVersionKind) (bool, error) {
	res, err := k.findAPIResourceForKind(kind)
	if err != nil {
		return false, err
	}

	return res.Namespaced, nil
}

// ResourceForKind returns the schema.GroupVersionResource corresponding to kind.
func (k *KubeClient) ResourceForKind(kind schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	res, err := k.findAPIResourceForKind(kind)
	if err != nil {
		return schema.GroupVersionResource{}, err
	}

	return schema.GroupVersionResource{
		Group:    res.Group,
		Version:  res.Version,
		Resource: res.Name,
	}, nil
}

// ResourcesForName returns the possible set of schema.GroupVersionResource
// corresponding to the given resource name.
func (k *KubeClient) ResourcesForName(name string) ([]schema.GroupVersionResource, error) {
	apiResources, err := k.ServerResources()
	if err != nil {
		return nil, err
	}

	var matched []schema.GroupVersionResource

	for _, r := range apiResources {
		if r.Name != name {
			continue
		}

		matched = append(matched, schema.GroupVersionResource{
			Group:    r.Group,
			Version:  r.Version,
			Resource: r.Name,
		})
	}

	return matched, nil
}

// SelectObjects lists the objects matching the given kind and selector.
func (k *KubeClient) SelectObjects(kind schema.GroupVersionKind, selector labels.Selector) (
	[]*unstructured.Unstructured, error) {
	res, err := k.findAPIResourceForKind(kind)
	if err != nil {
		return nil, err
	}

	r := schema.GroupVersionResource{
		Group:    res.Group,
		Version:  res.Version,
		Resource: res.Name,
	}

	var results []*unstructured.Unstructured

	// TODO(jpeach): set a more reasonable limit and implement Continue.
	list, err := k.Dynamic.Resource(r).Namespace(metav1.NamespaceAll).List(
		context.Background(), metav1.ListOptions{LabelSelector: selector.String(), Limit: 10000})

	if apierrors.IsNotFound(err) {
		return results, nil
	}

	if err != nil {
		return nil, err
	}

	for _, u := range list.Items {
		results = append(results, u.DeepCopy())
	}

	return results, nil
}

// ServerResources returns the list of all the resources supported
// by the API server. Note that this method guarantees to populate the
// Group and Version fields in the result.
func (k *KubeClient) ServerResources() ([]metav1.APIResource, error) {
	var resources []metav1.APIResource

	_, apiList, err := k.Discovery.ServerGroupsAndResources()
	if err != nil {
		return nil, err
	}

	for _, g := range apiList {
		// The listed resources will have empty Group and Version
		// fields, which means that they are the same as that of the
		// list. Parse the list's GroupVersion to populate the result.
		gv := must.GroupVersion(schema.ParseGroupVersion(g.GroupVersion))

		for _, r := range g.APIResources {

			r.Group = gv.Group
			r.Version = gv.Version
			resources = append(resources, r)
		}
	}

	return resources, nil
}

// SelectObjectsByLabel lists all objects that are labeled as managed.
func (k *KubeClient) SelectObjectsByLabel(label string, value string) ([]*unstructured.Unstructured, error) {
	groups, err := k.Discovery.ServerPreferredResources()
	if err != nil {
		return nil, err
	}

	var resources []schema.GroupVersionResource

	for _, g := range groups {
		// The listed resources will have empty Group and Version
		// fields, which means that they are the same as that of the
		// list. Parse the list's GroupVersion to populate the result.
		gv := must.GroupVersion(schema.ParseGroupVersion(g.GroupVersion))

		for _, r := range g.APIResources {
			// Only choose resources we can list.
			if !utils.ContainsString(r.Verbs, "list") {
				continue
			}

			gvr := schema.GroupVersionResource{
				Group:    gv.Group,
				Version:  gv.Version,
				Resource: r.Name,
			}

			resources = append(resources, gvr)
		}
	}

	selector := labels.SelectorFromSet(labels.Set{label: value}).String()

	var results []*unstructured.Unstructured

	for _, r := range resources {
		// TODO(jpeach): set a more reasonable limit and implement Continue.
		list, err := k.Dynamic.Resource(r).Namespace(metav1.NamespaceAll).List(
			context.Background(), metav1.ListOptions{LabelSelector: selector, Limit: 10000})

		if apierrors.IsNotFound(err) {
			continue
		}

		if err != nil {
			return nil, err
		}

		for _, u := range list.Items {
			results = append(results, u.DeepCopy())
		}
	}

	return results, nil
}

// RunIDFor returns the test run ID for u, if there is one. If there
// is no run ID, it returns "".
func (k *KubeClient) RunIDFor(u *unstructured.Unstructured) (string, error) {
	for k, v := range u.GetAnnotations() {
		if k == filter.LabelRunID {
			return v, nil
		}
	}

	// If this object doesn't have th run ID, walk up the owner
	// refs to try to find it.
	for range u.GetOwnerReferences() {
		// TODO(jpeach) ...
	}

	return "", nil
}

// NewKubeClient returns a new set of Kubernetes client interfaces
// that are configured to use the default Kubernetes context.
func NewKubeClient() (*KubeClient, error) {
	rules := clientcmd.NewDefaultClientConfigLoadingRules()
	overrides := &clientcmd.ConfigOverrides{}
	config := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(rules, overrides)

	restConfig, err := config.ClientConfig()
	if err != nil {
		return nil, err
	}

	clientSet, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	dynamicIntf, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		return nil, err
	}

	return &KubeClient{
		Config:    restConfig,
		Client:    clientSet,
		Dynamic:   dynamicIntf,
		Discovery: memory.NewMemCacheClient(clientSet.Discovery()),
	}, nil
}

// NewNamespace returns a v1/Namespace object named by nsName and
// converted to an unstructured.Unstructured object.
func NewNamespace(nsName string) *unstructured.Unstructured {
	ns := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}

	u := &unstructured.Unstructured{}

	if err := scheme.Scheme.Convert(ns, u, nil); err != nil {
		log.Fatalf("namespace conversion failed: %s", err)
	}

	return u
}

// ObjectReference uniquely identifies Kubernetes API object.
type ObjectReference struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace"`

	Meta struct {
		Group   string `json:"group"`
		Version string `json:"version"`
		Kind    string `json:"kind"`
	} `json:"meta"`
}

// FromUnstructured initializes an ObjectReference from a
// unstructured.Unstructured object.
func (o *ObjectReference) FromUnstructured(u *unstructured.Unstructured) *ObjectReference {

	o.Name = u.GetName()
	o.Namespace = u.GetNamespace()

	// We manually construct a GVK so that we can apply JSON
	// field labels to lowercase the names in the Rego data store.
	kind := u.GetObjectKind().GroupVersionKind()
	o.Meta.Group = kind.Group
	o.Meta.Version = kind.Version
	o.Meta.Kind = kind.Kind

	return o
}
