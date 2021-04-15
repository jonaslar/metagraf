/*
Copyright 2020 The metaGraf Authors

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
	"github.com/laetho/metagraf/pkg/metagraf"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapp "sigs.k8s.io/application/pkg/apis/app/v1beta1"
)

func GenApplication(mg *metagraf.MetaGraf) {

	obj := kapp.Application{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Application",
			APIVersion: "v1beta1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:         Name(mg),
			GenerateName: "",
			Namespace:    NameSpace,
			Labels:       mg.Metadata.Labels,
			Annotations:  mg.Metadata.Annotations,
		},
		Spec: kapp.ApplicationSpec{
			ComponentGroupKinds: mg.GroupKinds(),
			Descriptor: kapp.Descriptor{
				Type:        mg.Spec.Type,
				Version:     mg.Spec.Version,
				Description: mg.Spec.Description,
				Maintainers: nil,
				Owners:      nil,
				Keywords:    nil,
				Links:       nil,
				Notes:       "",
			},
			Selector: &metav1.LabelSelector{
				MatchLabels:      nil,
				MatchExpressions: nil,
			},
			AddOwnerRef:   false,
			Info:          nil,
			AssemblyPhase: "",
		},
		Status: kapp.ApplicationStatus{},
	}

	if !Dryrun {
		StoreApplication(obj)
	}
	if Output {
		MarshalObject(obj.DeepCopyObject())
	}
}

func StoreApplication(obj kapp.Application) {

}
