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

package persistence

import (
	"github.com/mmerrill3/persistence-operator/pkg/client/monitoring/v1alpha1"
	"github.com/pkg/errors"
	"github.com/prometheus/client_golang/prometheus"
	batch "k8s.io/client-go/pkg/apis/batch/v1"
	extensionsobj "k8s.io/client-go/pkg/apis/extensions/v1beta1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"time"
)

const (
	tprPersistenceInstance = "persistence-instance." + v1alpha1.TPRGroup
	tprPersistenceAction   = "persistence-action." + v1alpha1.TPRGroup
	configFilename         = "persistence.yaml"
	resyncPeriod           = 5 * time.Minute
)

// Operator manages persistence actions
type Operator struct {
	kclient              *kubernetes.Clientset
	mclient              *v1alpha1.MonitoringV1alpha1Client
	promInf              cache.SharedIndexInformer
	persistenceActionInf cache.SharedIndexInformer
	host                 string
	config               Config
}

// Config defines configuration parameters for the Operator.
type Config struct {
	Host                      string
	KubeletObject             string
	TLSInsecure               bool
	TLSConfig                 rest.TLSClientConfig
	ConfigReloaderImage       string
	PersistenceConfigReloader string
}

// PersistenceActionStatus evaluates the current status of a PersistenceAction deployment.  It return the status
func PersistenceActionStatus(kclient kubernetes.Interface, p *v1alpha1.PersistenceAction) (*v1alpha1.PersistenceActionStatus, error) {
	res := &v1alpha1.PersistenceActionStatus{}

	if p.Spec.applied == true {
		res.applied = true
	} else {
		jobs, err := kclient.Batch().Jobs(p.Namespace).List(ListOptions(p.Name))

		//go through the jobs, find one that is complete
		for _, job := range jobs.Items {
			if nil != job.Status.StartTime {
				res.ExecutionTime = job.Status.StartTime
			}
			if nil != job.completionTime {
				res.completionTime = job.completionTime
			}
			if job.succeeded > 0 {
				res.applied = true
			}
		}
	}

	return res, nil
}

func (c *Operator) RegisterMetrics(r prometheus.Registerer) {
	r.MustRegister(NewPrometheusCollector(c.promInf.GetStore()))
}

// Run the controller.
func (c *Operator) Run(stopc <-chan struct{}) error {
	errChan := make(chan error)
	go func() {
		v, err := c.kclient.Discovery().ServerVersion()
		if err != nil {
			errChan <- errors.Wrap(err, "communicating with server failed")
			return
		}
		glog.Infof("connection established, server version is %s", v)

		if err := c.createTPRs(); err != nil {
			errChan <- errors.Wrap(err, "creating TPRs failed")
			return
		}
		errChan <- nil
	}()

	select {
	case err := <-errChan:
		if err != nil {
			return err
		}
		glog.Info("TPR API endpoints ready")
	case <-stopc:
		return nil
	}

	go c.promInf.Run(stopc)
	go c.persistenceActionInf.Run(stopc)

	<-stopc
	return nil
}

func (c *Operator) createTPRs() error {

	tprs := []*extensionsobj.ThirdPartyResource{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: tprPersistenceInstance,
			},
			Versions: []extensionsobj.APIVersion{
				{Name: v1alpha1.TPRVersion},
			},
			Description: "Persistence Instance",
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name: tprPersistenceAction,
			},
			Versions: []extensionsobj.APIVersion{
				{Name: v1alpha1.TPRVersion},
			},
			Description: "Persistence Action",
		},
	}
	tprClient := c.kclient.Extensions().ThirdPartyResources()

	for _, tpr := range tprs {
		if _, err := tprClient.Create(tpr); err != nil && !apierrors.IsAlreadyExists(err) {
			return err
		}
		glog.Infof("TPR created : %s ", tpr.Name)
	}

	// We have to wait for the TPRs to be ready. Otherwise the initial watch may fail.
	err := k8sutil.WaitForTPRReady(c.kclient.CoreV1().RESTClient(), v1alpha1.TPRGroup, v1alpha1.TPRVersion, v1alpha1.TPRPersistenceInstanceName)
	if err != nil {
		return err
	}
	return k8sutil.WaitForTPRReady(c.kclient.CoreV1().RESTClient(), v1alpha1.TPRGroup, v1alpha1.TPRVersion, v1alpha1.TPRPersistenceActionName)
}
