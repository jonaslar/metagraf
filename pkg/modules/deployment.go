/*
Copyright 2018-2021 The metaGraf Authors

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

package modules

import (
	"context"
	"github.com/golang/glog"
	"github.com/laetho/metagraf/internal/pkg/affinity"
	"github.com/laetho/metagraf/internal/pkg/helpers"
	"github.com/laetho/metagraf/internal/pkg/k8sclient"
	"github.com/laetho/metagraf/internal/pkg/params"
	"github.com/laetho/metagraf/pkg/metagraf"
	"github.com/spf13/viper"
	"k8s.io/apimachinery/pkg/util/intstr"
	"strconv"
	"strings"

	//corev1 "k8s.io/api/core/v1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GenDeployment(mg *metagraf.MetaGraf, namespace string) {
	objname := Name(mg)

	registry := viper.GetString("registry")
	// If container registry host is set and it differs from default, use value from -r (--registry) flag.
	if len(Registry) > 0 && registry != Registry {
		registry = Registry
	}

	// If ImageNS is not provided, default to current NameSpace value
	if len(ImageNS) == 0 {
		ImageNS = NameSpace
	}

	// Resource labels
	l := make(map[string]string)
	l["app"] = objname
	l["deployment"] = objname

	// Selector
	sm := make(map[string]string)
	sm["app"] = objname
	sm["deployment"] = objname

	s := metav1.LabelSelector{
		MatchLabels: sm,
	}

	var RevisionHistoryLimit int32 = 5

	var MaxSurge intstr.IntOrString
	MaxSurge.StrVal = "25%"
	MaxSurge.Type = 1
	var MaxUnavailable intstr.IntOrString
	MaxUnavailable.StrVal = "25%"
	MaxUnavailable.Type = 1

	// Instance of RollingDeploymentStrategyParams
	rollingParams := appsv1.RollingUpdateDeployment{
		MaxSurge:       &MaxSurge,
		MaxUnavailable: &MaxUnavailable,
	}

	// Containers
	var Containers []corev1.Container
	var ContainerPorts []corev1.ContainerPort
	//var ContainerVolumes []string
	var Volumes []corev1.Volume
	var VolumeMounts []corev1.VolumeMount
	// Environment
	var EnvVars []corev1.EnvVar

	// ImageInfo := helpers.SkopeoImageInfo(DockerImage)
	HasImageInfo := false
	ImageInfo, err := helpers.ImageInfo(mg)
	if err != nil {
		HasImageInfo = false
	} else {
		HasImageInfo = true
	}

	EnvVars = GetEnvVars(mg, Variables)

	// Environment Variables from baserunimage
	if BaseEnvs && HasImageInfo {
		for _, e := range ImageInfo.Config.Env {
			es := strings.Split(e, "=")
			if helpers.SliceInString(EnvBlacklistFilter, strings.ToLower(es[0])) {
				continue
			}
			EnvVars = append(EnvVars, corev1.EnvVar{Name: es[0], Value: es[1]})
		}
	}

	// ContainerPorts
	if HasImageInfo {
		for k := range ImageInfo.Config.ExposedPorts {
			ss := strings.Split(k, "/")
			port, _ := strconv.Atoi(ss[0])
			ContainerPort := corev1.ContainerPort{
				ContainerPort: int32(port),
				Protocol:      corev1.Protocol(strings.ToUpper(ss[1])),
			}
			ContainerPorts = append(ContainerPorts, ContainerPort)
		}

		Volumes, VolumeMounts = volumes(mg, ImageInfo)
	}
	// Tying Container PodSpec together
	Container := corev1.Container{
		Name:            objname,
		Image:           imageRef(mg),
		ImagePullPolicy: corev1.PullIfNotPresent,
		Ports:           ContainerPorts,
		VolumeMounts:    VolumeMounts,
		Env:             EnvVars,
	}
	// Checking for Probes
	probe := corev1.Probe{}
	if mg.Spec.ReadinessProbe != probe {
		Container.ReadinessProbe = &mg.Spec.ReadinessProbe
	}
	if mg.Spec.LivenessProbe != probe {
		Container.LivenessProbe = &mg.Spec.LivenessProbe
	}
	if mg.Spec.StartupProbe != probe {
		Container.StartupProbe = &mg.Spec.StartupProbe
	}
	Containers = append(Containers, Container)

	// Tying the DeploymentObject together, literally :)
	obj := appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   objname,
			Labels: l,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas:             &params.Replicas,
			RevisionHistoryLimit: &RevisionHistoryLimit,
			Selector:             &s,
			Strategy: appsv1.DeploymentStrategy{
				Type:          appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &rollingParams,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Name:   objname,
					Labels: l,
				},
				Spec: corev1.PodSpec{
					Containers: Containers,
					Volumes:    Volumes,
				},
			},
		},
		Status: appsv1.DeploymentStatus{},
	}

	if params.WithAffinityRules {
		obj.Spec.Template.Spec.Affinity = affinity.SoftPodAntiAffinity(objname, params.PodAntiAffinityTopologyKey, params.PodAntiAffinityWeight)
	}

	if !Dryrun {
		StoreDeployment(obj)
	}
	if Output {
		MarshalObject(obj.DeepCopyObject())
	}
}

func StoreDeployment(obj appsv1.Deployment) {

	glog.Infof("ResourceVersion: %v Length: %v", obj.ResourceVersion, len(obj.ResourceVersion))
	glog.Infof("Namespace: %v", NameSpace)

	client := k8sclient.GetKubernetesClient().AppsV1().Deployments(NameSpace)
	if len(obj.ResourceVersion) > 0 {
		// update
		result, err := client.Update(context.TODO(), &obj, metav1.UpdateOptions{})
		if err != nil {
			glog.Info(err)
		}
		glog.Infof("Updated Deployment: %v(%v)", result.Name, obj.Name)
	} else {
		result, err := client.Create(context.TODO(), &obj, metav1.CreateOptions{})
		if err != nil {
			glog.Info(err)
		}
		glog.Infof("Created Deployment: %v(%v)", result.Name, obj.Name)
	}
}
