/*
Copyright 2021 The metaGraf Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package pdb

import (
	"bytes"
	"context"
	gojson "encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/golang/glog"
	k8sclient "github.com/laetho/metagraf/internal/pkg/k8sclient"
	params "github.com/laetho/metagraf/internal/pkg/params"
	"github.com/laetho/metagraf/pkg/metagraf"
	"github.com/laetho/metagraf/pkg/modules"
	"gopkg.in/yaml.v3"
	"k8s.io/api/policy/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/util/intstr"
	log "k8s.io/klog"
)

func GenDefaultPodDisruptionBudget(mg *metagraf.MetaGraf) v1beta1.PodDisruptionBudget {
	name := modules.Name(mg) // @todo refactor how we create a name.

	l := make(map[string]string)
	l["app"] = name

	selector := metav1.LabelSelector{
		MatchLabels: l,
	}

	obj := v1beta1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "policy/v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
			Namespace: params.NameSpace,
		},
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MinAvailable: &intstr.IntOrString{
				Type:   0,
				IntVal: 1,
			},
			Selector: &selector,
		},
	}

	if !params.Dryrun {
		StorePodDisruptionBudget(obj)
	}
	if params.Output {
		MarshalObject(obj.DeepCopyObject())
	}
	return obj
}

func GenPodDisruptionBudget(mg *metagraf.MetaGraf, replicas int32) v1beta1.PodDisruptionBudget {
	name := modules.Name(mg) // @todo refactor how we create a name.

	var maxunavail int32

	switch {
	case replicas < 2:
		maxunavail = 0
	case replicas >= 2:
		maxunavail = int32(math.Floor(float64(replicas / 2)))
	}

	l := make(map[string]string)
	l["app"] = name

	selector := metav1.LabelSelector{
		MatchLabels: l,
	}

	obj := v1beta1.PodDisruptionBudget{
		TypeMeta: metav1.TypeMeta{
			Kind:       "PodDisruptionBudget",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: name,
			CreationTimestamp: metav1.Time{
				Time: time.Now(),
			},
		},
		Spec: v1beta1.PodDisruptionBudgetSpec{
			MaxUnavailable: &intstr.IntOrString{
				Type:   0,
				IntVal: maxunavail,
			},
			Selector: &selector,
		},
	}

	if !params.Dryrun {
		StorePodDisruptionBudget(obj)
	}
	if params.Output {
		MarshalObject(obj.DeepCopyObject())
	}
	return obj
}

func StorePodDisruptionBudget(obj v1beta1.PodDisruptionBudget) {

	glog.Infof("ResourceVersion: %v Length: %v", obj.ResourceVersion, len(obj.ResourceVersion))
	glog.Infof("Namespace: %v", params.NameSpace)

	client := k8sclient.GetKubernetesClient().PolicyV1beta1().PodDisruptionBudgets(params.NameSpace)
	if len(obj.ResourceVersion) > 0 {
		// update
		result, err := client.Update(context.TODO(), &obj, metav1.UpdateOptions{})
		if err != nil {
			glog.Info(err)
		}
		glog.Infof("Updated: %v(%v)", result.Name, obj.Name)
	} else {
		result, err := client.Create(context.TODO(), &obj, metav1.CreateOptions{})
		if err != nil {
			glog.Info(err)
		}
		glog.Infof("Created: %v(%v)", result.Name, obj.Name)
	}
}

// todo: need to restructure code, this is a duplication
// Marshal kubernetes resource to json
func MarshalObject(obj runtime.Object) {
	opt := json.SerializerOptions{
		Yaml:   false,
		Pretty: true,
		Strict: true,
	}
	s := json.NewSerializerWithOptions(json.DefaultMetaFactory, nil, nil, opt)

	var buff bytes.Buffer
	err := s.Encode(obj.DeepCopyObject(), &buff)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}
	jsonMap := make(map[string]interface{})
	err = gojson.Unmarshal(buff.Bytes(), &jsonMap)
	if err != nil {
		panic(err)
	}

	delete(jsonMap, "status")

	switch params.Format {
	case "json":
		oj, err := gojson.MarshalIndent(jsonMap, "", "  ")
		if err != nil {
			panic(err)
		}
		fmt.Println(string(oj))
		return
	case "yaml":
		oy, err := yaml.Marshal(jsonMap)
		if err != nil {
			panic(err)
		}
		fmt.Println(string(oy))
	}
}
