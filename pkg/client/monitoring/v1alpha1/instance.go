// Copyright 2017 The persistence-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package v1alpha1

import (
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

const (
	TPRPersistenceInstancesKind = "PersistenceInstance"
	TPRPersistenceInstanceName  = "persistenceinstances"
)

type PersistenceInstanceGetter interface {
	PersistenceInstances(namespace string) PersistenceInstanceInterface
}

type PersistenceInstanceInterface interface {
	Create(*ServiceMonitor) (*PersistenceInstance, error)
	Get(name string) (*PersistenceInstance, error)
	Update(*PersistenceInstance) (*PersistenceInstance, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

type persistenceinstances struct {
	restClient rest.Interface
	client     *dynamic.ResourceClient
	ns         string
}

func newPersistenceInstances(r rest.Interface, c *dynamic.Client, namespace string) *persistenceinstances {
	return &persistenceinstances{
		r,
		c.Resource(
			&metav1.APIResource{
				Kind:       TPRPersistenceInstancesKind,
				Name:       TPRPersistenceInstanceName,
				Namespaced: true,
			},
			namespace,
		),
		namespace,
	}
}

func (s *persistenceinstances) Create(o *PersistenceInstance) (*PersistenceInstance, error) {
	us, err := UnstructuredFromPersistenceInstance(o)
	if err != nil {
		return nil, err
	}

	us, err = s.client.Create(us)
	if err != nil {
		return nil, err
	}

	return PersistenceInstanceFromUnstructured(us)
}

func (s *persistenceinstances) Get(name string) (*PersistenceInstance, error) {
	obj, err := s.client.Get(name)
	if err != nil {
		return nil, err
	}
	return PersistenceInstanceFromUnstructured(obj)
}

func (s *persistenceinstances) Update(o *PersistenceInstance) (*PersistenceInstance, error) {
	us, err := UnstructuredFromPersistenceInstance(o)
	if err != nil {
		return nil, err
	}

	us, err = s.client.Update(us)
	if err != nil {
		return nil, err
	}

	return PersistenceInstanceFromUnstructured(us)
}

func (s *persistenceinstances) Delete(name string, options *metav1.DeleteOptions) error {
	return s.client.Delete(name, options)
}

func (s *persistenceinstances) List(opts metav1.ListOptions) (runtime.Object, error) {
	req := s.restClient.Get().
		Namespace(s.ns).
		Resource("persistenceinstances").
		FieldsSelectorParam(nil)

	b, err := req.DoRaw()
	if err != nil {
		return nil, err
	}
	var pi PersistenceInstanceList
	return &pi, json.Unmarshal(b, &pi)
}

func (s *persistenceinstances) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	r, err := s.restClient.Get().
		Prefix("watch").
		Namespace(s.ns).
		Resource("persistenceinstances").
		FieldsSelectorParam(nil).
		Stream()
	if err != nil {
		return nil, err
	}
	return watch.NewStreamWatcher(&persistenceInstanceDecoder{
		dec:   json.NewDecoder(r),
		close: r.Close,
	}), nil
}

// PersistenceInstanceFromUnstructured unmarshals a PersistenceInstance object from dynamic client's unstructured
func PersistenceInstanceFromUnstructured(r *unstructured.Unstructured) (*PersistenceInstance, error) {
	b, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	var s PersistenceInstance
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	s.TypeMeta.Kind = TPRPersistenceInstancesKind
	s.TypeMeta.APIVersion = TPRGroup + "/" + TPRVersion
	return &s, nil
}

// UnstructuredFromPersistenceInstance marshals a PersistenceInstance object into dynamic client's unstructured
func UnstructuredFromPersistenceInstance(s *PersistenceInstance) (*unstructured.Unstructured, error) {
	s.TypeMeta.Kind = TPRPersistenceInstancesKind
	s.TypeMeta.APIVersion = TPRGroup + "/" + TPRVersion
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	var r unstructured.Unstructured
	if err := json.Unmarshal(b, &r.Object); err != nil {
		return nil, err
	}
	return &r, nil
}

type persistenceInstanceDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *persistenceInstanceDecoder) Close() {
	d.close()
}

func (d *persistenceInstanceDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e struct {
		Type   watch.EventType
		Object PersistenceInstance
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}
