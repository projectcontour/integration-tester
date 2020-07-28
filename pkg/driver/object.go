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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/projectcontour/integration-tester/pkg/must"
	"github.com/projectcontour/integration-tester/pkg/utils"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
)

// DefaultResyncPeriod is the default informer resync interval.
const DefaultResyncPeriod = time.Minute * 5

// OperationResult describes the result of an attempt to apply a
// Kubernetes object update.
type OperationResult struct {
	Error  *metav1.Status             `json:"error"`
	Latest *unstructured.Unstructured `json:"latest"`
	Target ObjectReference            `json:"target"`
}

// Succeeded returns true if the operation was successful.
func (o *OperationResult) Succeeded() bool {
	return o.Error == nil
}

// ObjectDriver is a driver that is responsible for the lifecycle
// of Kubernetes API documents, expressed as unstructured.Unstructured
// objects.
type ObjectDriver interface {
	// Eval creates or updates the specified object.
	Apply(*unstructured.Unstructured) (*OperationResult, error)

	// Delete deleted the specified object.
	Delete(*unstructured.Unstructured) (*OperationResult, error)

	// Adopt tells the driver to take ownership of and to start tracking
	// the specified object. Any adopted objects will be included in a
	// DeleteAll operation.
	Adopt(*unstructured.Unstructured) error

	// DeleteAll deletes all the objects that have been adopted by this driver.
	DeleteAll() error

	// InformOn establishes an informer for the given resource.
	// Events received by this informer will be delivered to all
	// watchers.
	InformOn(gvr schema.GroupVersionResource) error

	// WaitForCacheSync waits until all the informers created
	// by the driver have synced.
	WaitForCacheSync(timeout time.Duration) error

	// Watch registers an event handler to receive events from
	// all the informers managed by the driver.
	Watch(cache.ResourceEventHandler) func()

	// Done marks this driver session as complete. All informers
	// are released, watchers are unregistered and adopted objects
	// are forgotten.
	Done()
}

// NewObjectDriver returns a new ObjectDriver.
func NewObjectDriver(client *KubeClient) ObjectDriver {
	// We used to inform with a managed-by=integration-tester filter
	// so that we would only track objects that we create ourselves.
	// However, in some cases, it is impossible to propagate labels
	// down the object tree because the top-level object that we
	// create doesn't spec a template that can be used to apply
	// labels. So, we basically have to just watch everything
	// by type.

	options := dynamicinformer.TweakListOptionsFunc(
		func(o *metav1.ListOptions) {})

	factory := dynamicinformer.NewFilteredDynamicSharedInformerFactory(
		client.Dynamic,
		DefaultResyncPeriod,
		metav1.NamespaceAll,
		options,
	)

	o := &objectDriver{
		kube:            client,
		informerStopper: make(chan struct{}),
		informerFactory: factory,

		// watcherLock holds a lock over the watchers because
		// we need to ensure watcher add and remove operations
		// are serialized WRT event delivery.
		watcherLock: LockingResourceEventHandler{
			Next: &MuxingResourceEventHandler{},
		},

		objectPool:   make(map[types.UID]*unstructured.Unstructured),
		informerPool: make(map[schema.GroupVersionResource]informers.GenericInformer),
	}

	return o
}

var _ ObjectDriver = &objectDriver{}

type objectDriver struct {
	kube *KubeClient

	informerStopper chan struct{}
	informerFactory dynamicinformer.DynamicSharedInformerFactory

	watcherLock LockingResourceEventHandler

	informerPool map[schema.GroupVersionResource]informers.GenericInformer

	objectLock sync.Mutex
	objectPool map[types.UID]*unstructured.Unstructured
}

// Done resets the object driver.
func (o *objectDriver) Done() {
	// Tell any informers to shut down.
	close(o.informerStopper)

	// Hold the watcher lock while we clear the watchers.
	o.watcherLock.Lock.Lock()
	o.watcherLock.Next.(*MuxingResourceEventHandler).Clear()
	o.watcherLock.Lock.Unlock()

	// Hold the object lock while we clear the object pool.
	o.objectLock.Lock()
	o.objectPool = make(map[types.UID]*unstructured.Unstructured)
	o.objectLock.Unlock()

	// There is no locking on the informer pool since driver
	// methods must not be called concurrently.
	o.informerPool = make(map[schema.GroupVersionResource]informers.GenericInformer)
}

func (o *objectDriver) Watch(e cache.ResourceEventHandler) func() {
	o.watcherLock.Lock.Lock()
	defer o.watcherLock.Lock.Unlock()

	which := o.watcherLock.Next.(*MuxingResourceEventHandler).Add(e)

	return func() {
		o.watcherLock.Lock.Lock()
		defer o.watcherLock.Lock.Unlock()

		o.watcherLock.Next.(*MuxingResourceEventHandler).Remove(which)
	}
}

func (o *objectDriver) InformOn(gvr schema.GroupVersionResource) error {
	if _, ok := o.informerPool[gvr]; ok {
		return nil
	}

	// If we don't already have an informer for this resource, start one now.
	genericInformer := o.informerFactory.ForResource(gvr)
	genericInformer.Informer().AddEventHandler(
		&WrappingResourceEventHandlerFuncs{
			Next: &o.watcherLock,
			AddFunc: func(obj interface{}) {
				o.objectLock.Lock()
				defer o.objectLock.Unlock()

				if u, ok := obj.(*unstructured.Unstructured); ok {
					o.updateAdoptedObject(u)
				}
			},
			UpdateFunc: func(oldObj, newObj interface{}) {
				o.objectLock.Lock()
				defer o.objectLock.Unlock()

				if u, ok := newObj.(*unstructured.Unstructured); ok {
					o.updateAdoptedObject(u)
				}
			},
			DeleteFunc: func(obj interface{}) {
				o.objectLock.Lock()
				defer o.objectLock.Unlock()

				if u, ok := obj.(*unstructured.Unstructured); ok {
					delete(o.objectPool, u.GetUID())
				}
			},
		})

	o.informerPool[gvr] = genericInformer

	go func() {
		genericInformer.Informer().Run(o.informerStopper)
	}()

	return nil
}

func (o *objectDriver) WaitForCacheSync(timeout time.Duration) error {
	var synced []cache.InformerSynced

	for _, i := range o.informerPool {
		synced = append(synced, i.Informer().HasSynced)
	}

	stopChan := make(chan struct{})
	timer := time.AfterFunc(timeout, func() { close(stopChan) })
	defer timer.Stop()

	if !cache.WaitForCacheSync(stopChan, synced...) {
		return errors.New("informer cache sync timed out")
	}

	return nil
}

func (o *objectDriver) Apply(obj *unstructured.Unstructured) (*OperationResult, error) {
	obj = obj.DeepCopy() // Copy in case we set the namespace.
	gvk := obj.GetObjectKind().GroupVersionKind()

	isNamespaced, err := o.kube.KindIsNamespaced(gvk)
	if err != nil {
		return nil, fmt.Errorf("failed check if resource kind %q is namespaced: %s",
			gvk.Kind, err)
	}

	gvr, err := o.kube.ResourceForKind(obj.GetObjectKind().GroupVersionKind())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource for kind %s:%s: %s",
			obj.GetAPIVersion(), obj.GetKind(), err)
	}

	if err := o.InformOn(gvr); err != nil {
		return nil, fmt.Errorf("failed to start informer for %q: %s", gvr, err)
	}

	if isNamespaced {
		if ns := obj.GetNamespace(); ns == "" {
			obj.SetNamespace(metav1.NamespaceDefault)
		}
	}

	var latest *unstructured.Unstructured

	if isNamespaced {
		latest, err = o.kube.Dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Create(obj, metav1.CreateOptions{})
	} else {
		latest, err = o.kube.Dynamic.Resource(gvr).Create(obj, metav1.CreateOptions{})
	}

	// If the create was against an object that already existed,
	// retry as an update.
	if apierrors.IsAlreadyExists(err) {
		name := obj.GetName()
		opt := metav1.PatchOptions{}
		ptype := types.MergePatchType
		data := must.Bytes(obj.MarshalJSON())

		// This is a hacky shortcut to emulate what kubectl
		// does in apply.Patcher. Since only built-in types
		// support strategic merge, we use the scheme check
		// to test whether this object is builtin or not.
		if _, err := scheme.Scheme.New(obj.GroupVersionKind()); err == nil {
			ptype = types.StrategicMergePatchType
		}

		if isNamespaced {
			latest, err = o.kube.Dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Patch(name, ptype, data, opt)
		} else {
			latest, err = o.kube.Dynamic.Resource(gvr).Patch(name, ptype, data, opt)
		}
	}

	result := OperationResult{
		Error:  nil,
		Latest: obj,
		Target: *(&ObjectReference{}).FromUnstructured(obj),
	}

	switch err {
	case nil:
		result.Latest = latest
		if err := o.Adopt(latest); err != nil {
			return nil, fmt.Errorf("failed to adopt %s %s/%s: %w",
				latest.GetKind(), latest.GetNamespace(), latest.GetName(), err)

		}

	default:
		var statusError *apierrors.StatusError
		if !errors.As(err, &statusError) {
			return nil, fmt.Errorf("failed to apply resource: %w", err)
		}

		result.Error = &statusError.ErrStatus
	}

	return &result, nil
}

func (o *objectDriver) Delete(obj *unstructured.Unstructured) (*OperationResult, error) {
	obj = obj.DeepCopy() // Copy in case we set the namespace.
	gvk := obj.GetObjectKind().GroupVersionKind()

	isNamespaced, err := o.kube.KindIsNamespaced(gvk)
	if err != nil {
		return nil, fmt.Errorf("failed check if resource kind is namespaced: %s", err)
	}

	gvr, err := o.kube.ResourceForKind(obj.GetObjectKind().GroupVersionKind())
	if err != nil {
		return nil, fmt.Errorf("failed to resolve resource for kind %s:%s: %s",
			obj.GetAPIVersion(), obj.GetKind(), err)
	}

	// Default the namespace before checking the object pool.
	if isNamespaced {
		if ns := obj.GetNamespace(); ns == "" {
			obj.SetNamespace(metav1.NamespaceDefault)
		}
	}

	result := OperationResult{
		Error:  nil,
		Latest: obj,
		Target: *(&ObjectReference{}).FromUnstructured(obj),
	}

	// Scan for the latest update if we have adopted this object.
	// The caller doesn't have to provide a complete object from
	// the API server, so we can't match on the UID here.
	//
	// TODO(jpeach): maybe we should just fetch the object first?
	o.objectLock.Lock()
	for _, adopted := range o.objectPool {
		if adopted.GetName() == obj.GetName() &&
			adopted.GetNamespace() == obj.GetNamespace() &&
			adopted.GetKind() == obj.GetKind() {

			result.Latest = adopted.DeepCopy()
			break
		}
	}
	o.objectLock.Unlock()

	opts := utils.ImmediateDeletionOptions(metav1.DeletePropagationForeground)

	// Services need to be deleted in the background, see
	//	https://github.com/kubernetes/kubernetes/issues/87603
	//	https://github.com/kubernetes/kubernetes/issues/90512
	if obj.GetKind() == "Service" {
		opts = utils.ImmediateDeletionOptions(metav1.DeletePropagationBackground)
	}

	if isNamespaced {
		err = o.kube.Dynamic.Resource(gvr).Namespace(obj.GetNamespace()).Delete(obj.GetName(), opts)
	} else {
		err = o.kube.Dynamic.Resource(gvr).Delete(obj.GetName(), opts)
	}

	switch err {
	case nil:
		result.Error = nil
	default:
		var statusError *apierrors.StatusError
		if !errors.As(err, &statusError) {
			return nil, fmt.Errorf("failed to apply resource: %w", err)
		}

		result.Error = &statusError.ErrStatus
	}

	return &result, nil
}

func (o *objectDriver) updateAdoptedObject(obj *unstructured.Unstructured) {
	uid := obj.GetUID()

	// Update our adopted object only if it is from a newer generation.
	if prev, ok := o.objectPool[uid]; ok {
		if obj.GetGeneration() > prev.GetGeneration() {
			o.objectPool[uid] = obj.DeepCopy()
		}
	}
}

func (o *objectDriver) Adopt(obj *unstructured.Unstructured) error {
	o.objectLock.Lock()
	defer o.objectLock.Unlock()

	uid := obj.GetUID()

	// We can't adopt any object that hasn't come back from the
	// API server, since it isn't a legit object until then.
	if uid == "" {
		return errors.New("no object UID")
	}

	// Update our adopted object only if it is from a newer generation.
	if prev, ok := o.objectPool[uid]; ok {
		if obj.GetGeneration() > prev.GetGeneration() {
			o.objectPool[uid] = obj.DeepCopy()
		}
	} else {
		o.objectPool[uid] = obj.DeepCopy()
	}

	return nil
}

func (o *objectDriver) DeleteAll() error {
	for {
		var errs []error
		targets := make([]*unstructured.Unstructured, 0, len(o.objectPool))

		o.objectLock.Lock()
		for _, u := range o.objectPool {
			targets = append(targets, u.DeepCopy())
		}
		o.objectLock.Unlock()

		if len(targets) == 0 {
			return nil
		}

		for _, u := range targets {
			result, err := o.Delete(u)

			if err != nil {
				errs = append(errs, err)
				continue
			}

			if result.Error != nil {
				switch result.Error.Reason {
				case metav1.StatusReasonNotFound, metav1.StatusReasonGone:
					// If the deletion failed because the target wasn't there, then the object
					// pool won't get updated by the informer callback. We have to update it here.
					o.objectLock.Lock()
					delete(o.objectPool, u.GetUID())
					o.objectLock.Unlock()
				default:
					// Re-wrap the error that we unwrapped for status!
					errs = append(errs, &apierrors.StatusError{
						ErrStatus: *result.Error,
					})
					continue
				}
			}
		}

		if len(errs) != 0 {
			errs = append([]error{errors.New("failed to delete all objects")}, errs...)
			return utils.ChainErrors(errs...)
		}

		time.Sleep(time.Second)
	}
}
