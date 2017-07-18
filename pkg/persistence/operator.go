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
	"fmt"
	"github.com/golang/glog"
	"github.com/mmerrill3/persistence-operator/pkg/client/persistence/v1alpha1"
	"github.com/mmerrill3/persistence-operator/pkg/k8sutil"
	"github.com/mmerrill3/persistence-operator/third_party/workqueue"
	"github.com/pkg/errors"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
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
	mclient              *v1alpha1.PersistenceV1alpha1Client
	persistenceActionInf cache.SharedIndexInformer
	host                 string
	config               Config
	queue                workqueue.RateLimitingInterface
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

	if p.Spec.Applied == true {
		res.Applied = true
	} else {
		jobs, err := kclient.Batch().Jobs(p.Namespace).List(ListOptions(p.Name))

		//go through the jobs, find one that is complete
		for _, job := range jobs.Items {
			if nil != job.Status.StartTime {
				res.ExecutionTime = job.Status.StartTime
			}
			if nil != job.Status.CompletionTime {
				res.CompletionTime = job.Status.CompletionTime
			}
			if job.Status.Succeeded > 0 {
				res.Applied = true
			}
		}
	}

	return res, nil
}

// New creates a new controller.
func New(conf Config) (*Operator, error) {
	cfg, err := k8sutil.NewClusterConfig(conf.Host, conf.TLSInsecure, &conf.TLSConfig)
	if err != nil {
		return nil, err
	}
	client, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	mclient, err := v1alpha1.NewForConfig(cfg)
	if err != nil {
		return nil, err
	}

	c := &Operator{
		kclient:                client,
		mclient:                mclient,
		host:                   cfg.Host,
		kubeletObjectName:      kubeletObjectName,
		kubeletObjectNamespace: kubeletObjectNamespace,
		config:                 conf,
		queue:                  workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "persistence"),
	}

	c.persistenceActionInf = cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc:  mclient.PersistenceActions(api.NamespaceAll).List,
			WatchFunc: mclient.PersistenceActions(api.NamespaceAll).Watch,
		},
		&v1alpha1.PersistenceAction{}, resyncPeriod, cache.Indexers{},
	)
	c.persistenceActionInf.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    c.handlePersistenceActionAdd,
		DeleteFunc: c.handlePersistenceActionDelete,
		UpdateFunc: c.handlePersistenceActionUpdate,
	})

	return c, nil
}

// Run the controller.
func (c *Operator) Run(stopc <-chan struct{}) error {
	defer c.queue.ShutDown()

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

	go c.worker()
	go c.persistenceActionInf.Run(stopc)

	<-stopc
	return nil
}

// worker runs a worker thread that just dequeues items, processes them, and marks them done.
// It enforces that the syncHandler is never invoked concurrently with the same key.
func (c *Operator) worker() {
	for c.processNextWorkItem() {
	}
}

func (c *Operator) processNextWorkItem() bool {
	key, quit := c.queue.Get()
	if quit {
		return false
	}
	defer c.queue.Done(key)

	err := c.sync(key.(string))
	if err == nil {
		c.queue.Forget(key)
		return true
	}

	utilruntime.HandleError(errors.Wrap(err, fmt.Sprintf("Sync %q failed", key)))
	c.queue.AddRateLimited(key)

	return true
}

func (c *Operator) sync(key string) error {
	obj, exists, err := c.promInf.GetIndexer().GetByKey(key)
	if err != nil {
		return err
	}
	if !exists {
		return c.destroyPersistenceAction(key)
	}

	p := obj.(*v1alpha1.PersistenceAction)
	if p.Spec.Applied {
		glog.V(7).Infof("PersistenceAction already applied: %s", key)
		return nil
	}

	glog.Infof("sync PersistenceAction", key)

	// Create CronJob if it doesn't exist.
	cronJobClient := c.kclient.BatchV2alpha1().CronJobs(p.Namespace)
	newCronJob, err := makeCronJob(p)
	if err != nil {
		return err
	}
	if err := k8sutil.CreateOrUpdateCronJob(cronJobClient, newCronJob); err != nil {
		return errors.Wrap(err, "synchronizing cron job failed")
	}

	return nil
}

func (c *Operator) destroyPersistenceAction(key string) error {
	// Create CronJob if it doesn't exist.
	cronJobClient := c.kclient.BatchV2alpha1().CronJobs(p.Namespace)
	if err := k8sutil.DeleteCronJob(svcClient, key); err != nil {
		return errors.Wrap(err, "Deleting cron job failed")
	}
	return nil
}

func (c *Operator) keyFunc(obj interface{}) (string, bool) {
	k, err := cache.DeletionHandlingMetaNamespaceKeyFunc(obj)
	if err != nil {
		glog.Errorf("creating key failed :%s", err)
		return k, false
	}
	return k, true
}

func (c *Operator) handlePersistenceActionAdd(obj interface{}) {
	key, ok := c.keyFunc(obj)
	if !ok {
		return
	}
	glog.Infof("Persistence added : %s", key)
	//TODO check if there is a job
	//if not, add one at the time the job is meant to run
	c.enqueue(key)
}

func (c *Operator) handlePersistenceActionDelete(obj interface{}) {
	key, ok := c.keyFunc(obj)
	if !ok {
		return
	}
	glogInfof("Persistence deleted : %s", key)
	//TODO check if there is a job
	//if so, remove it
	c.enqueue(key)
}

func (c *Operator) handlePersistenceActionUpdate(old, cur interface{}) {
	key, ok := c.keyFunc(cur)
	if !ok {
		return
	}
	glog.Infof("Persistence updated : %s", key)
	//TODO check if there is a job
	//if so, remove it, and add a new one
	c.enqueue(key)
}

// enqueue adds a key to the queue. If obj is a key already it gets added directly.
// Otherwise, the key is extracted via keyFunc.
func (c *Operator) enqueue(obj interface{}) {
	if obj == nil {
		return
	}

	key, ok := obj.(string)
	if !ok {
		key, ok = c.keyFunc(obj)
		if !ok {
			return
		}
	}

	c.queue.Add(key)
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

func ListOptions(name string) metav1.ListOptions {
	return metav1.ListOptions{
		LabelSelector: fields.SelectorFromSet(fields.Set(map[string]string{
			"app":        "prometheus",
			"prometheus": name,
		})).String(),
	}
}
