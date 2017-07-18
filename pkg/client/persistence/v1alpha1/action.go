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
	TPRPersistenceActionsKind = "PersistenceAction"
	TPRPersistenceActionName  = "persistenceactions"
)

type PersistenceActionGetter interface {
	PersistenceActions(namespace string) PersistenceActionInterface
}

type PersistenceActionInterface interface {
	Create(*PersistenceAction) (*PersistenceAction, error)
	Get(name string) (*PersistenceAction, error)
	Update(*PersistenceAction) (*PersistenceAction, error)
	Delete(name string, options *metav1.DeleteOptions) error
	List(opts metav1.ListOptions) (runtime.Object, error)
	Watch(opts metav1.ListOptions) (watch.Interface, error)
}

type persistenceactions struct {
	restClient rest.Interface
	client     *dynamic.ResourceClient
	ns         string
}

func newPersistenceActions(r rest.Interface, c *dynamic.Client, namespace string) *persistenceactions {
	return &persistenceactions{
		r,
		c.Resource(
			&metav1.APIResource{
				Kind:       TPRPersistenceActionsKind,
				Name:       TPRPersistenceActionName,
				Namespaced: true,
			},
			namespace,
		),
		namespace,
	}
}

func (s *persistenceactions) Create(o *PersistenceAction) (*PersistenceAction, error) {
	us, err := UnstructuredFromPersistenceAction(o)
	if err != nil {
		return nil, err
	}

	us, err = s.client.Create(us)
	if err != nil {
		return nil, err
	}

	return PersistenceActionFromUnstructured(us)
}

func (s *persistenceactions) Get(name string) (*PersistenceAction, error) {
	obj, err := s.client.Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	return PersistenceActionFromUnstructured(obj)
}

func (s *persistenceactions) Update(o *PersistenceAction) (*PersistenceAction, error) {
	us, err := UnstructuredFromPersistenceAction(o)
	if err != nil {
		return nil, err
	}

	us, err = s.client.Update(us)
	if err != nil {
		return nil, err
	}

	return PersistenceActionFromUnstructured(us)
}

func (s *persistenceactions) Delete(name string, options *metav1.DeleteOptions) error {
	return s.client.Delete(name, options)
}

func (s *persistenceactions) List(opts metav1.ListOptions) (runtime.Object, error) {
	req := s.restClient.Get().
		Namespace(s.ns).
		Resource("persistenceactions").
		FieldsSelectorParam(nil)

	b, err := req.DoRaw()
	if err != nil {
		return nil, err
	}
	var pi PersistenceActionList
	return &pi, json.Unmarshal(b, &pi)
}

func (s *persistenceactions) Watch(opts metav1.ListOptions) (watch.Interface, error) {
	r, err := s.restClient.Get().
		Prefix("watch").
		Namespace(s.ns).
		Resource("persistenceactions").
		FieldsSelectorParam(nil).
		Stream()
	if err != nil {
		return nil, err
	}
	return watch.NewStreamWatcher(&persistenceActionDecoder{
		dec:   json.NewDecoder(r),
		close: r.Close,
	}), nil
}

// PersistenceActionFromUnstructured unmarshals a PersistenceAction object from dynamic client's unstructured
func PersistenceActionFromUnstructured(r *unstructured.Unstructured) (*PersistenceAction, error) {
	b, err := json.Marshal(r.Object)
	if err != nil {
		return nil, err
	}
	var s PersistenceAction
	if err := json.Unmarshal(b, &s); err != nil {
		return nil, err
	}
	s.TypeMeta.Kind = TPRPersistenceActionsKind
	s.TypeMeta.APIVersion = TPRGroup + "/" + TPRVersion
	return &s, nil
}

// UnstructuredFromPersistenceAction marshals a PersistenceAction object into dynamic client's unstructured
func UnstructuredFromPersistenceAction(s *PersistenceAction) (*unstructured.Unstructured, error) {
	s.TypeMeta.Kind = TPRPersistenceActionsKind
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

type persistenceActionDecoder struct {
	dec   *json.Decoder
	close func() error
}

func (d *persistenceActionDecoder) Close() {
	d.close()
}

func (d *persistenceActionDecoder) Decode() (action watch.EventType, object runtime.Object, err error) {
	var e struct {
		Type   watch.EventType
		Object PersistenceAction
	}
	if err := d.dec.Decode(&e); err != nil {
		return watch.Error, nil, err
	}
	return e.Type, &e.Object, nil
}
