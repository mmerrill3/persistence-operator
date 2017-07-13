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

package api

import (
	"encoding/json"
	"net/http"
	"regexp"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"

	"github.com/coreos/persistence-operator/pkg/client/monitoring/v1alpha1"
	"github.com/coreos/persistence-operator/pkg/k8sutil"
	"github.com/coreos/persistence-operator/pkg/persistence"
)

type API struct {
	kclient *kubernetes.Clientset
	mclient *v1alpha1.MonitoringV1alpha1Client
}

func New(conf persistence.Config) (*API, error) {
	cfg, err := k8sutil.NewClusterConfig(conf.Host, conf.TLSInsecure, &conf.TLSConfig)
	if err != nil {
		return nil, err
	}

	kclient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	mclient, err := v1alpha1.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	return &API{
		kclient: kclient,
		mclient: mclient,
	}, nil
}

var (
	instanceRoute = regexp.MustCompile("/apis/persistence.mmerrill3.com/v1alpha1/namespaces/(.*)/persistence-actions/(.*)/status")
)

func (api *API) Register(mux *http.ServeMux) {
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if instanceRoute.MatchString(req.URL.Path) {
			api.status(w, req)
		} else {
			w.WriteHeader(404)
		}
	})
}

type objectReference struct {
	name      string
	namespace string
}

func parseStatusUrl(path string) objectReference {
	matches := instanceRoute.FindAllStringSubmatch(path, -1)
	ns := ""
	name := ""
	if len(matches) == 1 {
		if len(matches[0]) == 3 {
			ns = matches[0][1]
			name = matches[0][2]
		}
	}

	return objectReference{
		name:      name,
		namespace: ns,
	}
}

func (api *API) status(w http.ResponseWriter, req *http.Request) {
	or := parseStatusUrl(req.URL.Path)

	p, err := api.mclient.PersistenceAction(or.namespace).Get(or.name)
	if err != nil {
		if k8sutil.IsResourceNotFoundError(err) {
			w.WriteHeader(404)
		}
		glog.Errorf("Problem while fetching the action from k8s : %s", err)
		return
	}

	p.Status, _, err = persistence.PersistenceActionStatus(api.kclient, p)
	if err != nil {
		glog.Errorf("Problem while checking the status of the action from k8s : %s", err)
	}

	b, err := json.Marshal(p)
	if err != nil {
		glog.Errorf("Problem while marshalling the status of the action from k8s : %s", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(200)
	w.Write(b)
}
