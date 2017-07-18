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
	"github.com/mmerrill3/persistence-operator/pkg/client/persistence/v1alpha1"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/pkg/apis/batch/v2alpha1"
)

const (
	oracleBaseImage   = ""
	postgresBaseImage = ""
	mysqlBaseImage    = ""
	mongoBaseImage    = ""
)

func makeCronJob(p v1alpha1.PersistenceAction) (*v2alpha1.CronJob, error) {
	spec, err := makeCronJobSpec(p)
	if err != nil {
		return nil, errors.Wrap(err, "make CronJob spec")
	}
	p.ObjectMeta.Labels["persistence"] = p.Name
	cronjob := &v2alpha1.CronJob{
		ObjectMeta: metav1.ObjectMeta{
			Name:        prefixedName(p.Name),
			Labels:      p.ObjectMeta.Labels,
			Annotations: p.ObjectMeta.Annotations,
		},
		Spec: *spec,
	}

	return cronjob, nil
}

func makeCronJobSpec(p v1alpha1.PersistenceAction) (*v2alpha1.CronJobSpec, error) {
	return &v2alpha1.CronJobSpec{}, nil
}

func prefixedName(name string) string {
	return fmt.Sprintf("persistence-%s", name)
}
