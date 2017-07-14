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
	"time"
)

// defines a Persistence Instance
type PersistenceInstance struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the PersistenceInstance. More info:
	// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status
	Spec PersistenceInstanceSpec `json:"spec"`
}

// Specification of the desired behavior of the PersistenceAction. More info:
// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status
type PersistenceInstanceSpec struct {
	// One of Oracle, Postgres, MySQL, Mongo
	PersistenceType string `json:"persistenceType"`
	// The name of the secret containing the username, in the same namespace as the
	// PersistenceInstance
	UsernameSecret string `json:"usernameSecret,omitempty"`
	// The name of the secret containing the password, in the same namespace as the
	// PersistenceInstance
	PasswordSecret string `json:"passwordSecret,omitempty"`
	// The url of the persistence
	URL string `json:"url"`
	// The port of the persistence
	Port int32 `json:"port"`
}

// defines a Persistence Action
type PersistenceAction struct {
	metav1.TypeMeta `json:",inline"`
	// Standard object’s metadata. More info:
	// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#metadata
	metav1.ObjectMeta `json:"metadata,omitempty"`
	// Specification of the desired behavior of the PersistenceAction. More info:
	// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status
	Spec PersistenceActionSpec `json:"spec"`
	// Most recent observed status of the PersistenceAction. Read-only. Not
	// included when requesting from the apiserver, only from the Persistence
	// Operator API itself. More info:
	// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status
	Status *PersistenceActionStatus `json:"status,omitempty"`
}

// Specification of the desired behavior of the PersistenceAction. More info:
// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status
type PersistenceActionSpec struct {
	// ServiceMonitors to be selected for target discovery.
	PersistenceInstanceSelector *metav1.LabelSelector `json:"persistenceInstanceSelector,omitempty"`
	// Whether a persistence action is applied already.  Make it a no-operation
	Applied bool `json:"applied,omitempty"`
	// Define resources requests and limits for Pods.
	Resources v1.ResourceRequirements `json:"resources,omitempty"`
	// Define which Nodes the Pods are scheduled on.
	NodeSelector map[string]string `json:"nodeSelector,omitempty"`
	// ServiceAccountName is the name of the ServiceAccount to use to run the
	// PersistenceAction Pods.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// Define which tolerations are appicable for the pods
	Tolerations []v1.Toleration `json:"tolerations,omitempty"`
	// The time that the action will be ran
	ApplicationTime time `json:"applicationTime"`
	// The actual actions to run.  Every value in the map will be executed in keys order, ascending
	Actions map[string]string `json:"actions"`
}

// Most recent observed status of a PersistenceAction. Read-only. Not
// included when requesting from the apiserver, only from the Persistence
// Operator API itself. More info:
// http://releases.k8s.io/HEAD/docs/devel/api-conventions.md#spec-and-status
type PersistenceActionStatus struct {
	// Represents whether the action has been performed
	Applied bool `json:"paused"`
	// The time that the action started exectuion
	ExecutionTime time `json:"executionTime"`
	// The time that the action completed
	CompletionTime time `json:"completionTime"`
}
