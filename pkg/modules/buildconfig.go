/*
Copyright 2018-2020 The metaGraf Authors

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
	"fmt"
	"os"
	"strings"

	"github.com/laetho/metagraf/internal/pkg/helpers"
	"github.com/laetho/metagraf/internal/pkg/imageurl"
	"github.com/laetho/metagraf/internal/pkg/k8sclient"
	"github.com/laetho/metagraf/internal/pkg/params"
	log "k8s.io/klog"

	"github.com/laetho/metagraf/pkg/metagraf"

	buildv1 "github.com/openshift/api/build/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"github.com/openshift/oc/pkg/helpers/source-to-image/tar"
)

func TriggerLocalBuild(mg metagraf.MetaGraf) {
/*
	br := buildv1.BuildRequest{
		TypeMeta:              metav1.TypeMeta{},
		ObjectMeta:            metav1.ObjectMeta{},
	}
	br.Kind = "BuildRequest"
	br.APIVersion = "build.openshift.io/v1"
	br.ObjectMeta.Name = mg.Name("","")
	brt := buildv1.BuildTriggerCause{}
	brt.Message = "Triggered by mg."
	br.TriggeredBy = []buildv1.BuildTriggerCause{brt}
	client := k8sclient.GetBuildClient()
	build, err := client.BuildConfigs(params.NameSpace).Instantiate(context.TODO(), br.ObjectMeta.Name,&br, metav1.CreateOptions{})
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Started", build.Name)

	tar.
 */
}

func GenBuildConfig(mg *metagraf.MetaGraf) {
	var buildsource buildv1.BuildSource
	var imgurl imageurl.ImageURL
	var EnvVars []corev1.EnvVar

	err := imgurl.Parse(mg.Spec.BuildImage)
	if err != nil {

		log.Errorf("Malformed BuildImage url provided in metaGraf file; %v", mg.Spec.BuildImage)
		os.Exit(1)
	}

	objname := Name(mg)

	if len(mg.Spec.BaseRunImage) > 0 && len(mg.Spec.Repository) > 0 {
		buildsource = genBinaryBuildSource()
	} else if len(mg.Spec.BuildImage) > 0 && len(mg.Spec.BaseRunImage) < 1 {
		buildsource = genGitBuildSource(mg)
	}

	if BaseEnvs {
		log.V(2).Info("Populate environment variables form base image.")
		client := k8sclient.GetImageClient()

		ist := helpers.GetImageStreamTags(
			client,
			imgurl.Namespace,
			imgurl.Image+":"+imgurl.Tag)

		ImageInfo := helpers.GetDockerImageFromIST(ist)

		// Environment Variables from buildimage
		for _, e := range ImageInfo.Config.Env {
			es := strings.Split(e, "=")
			if helpers.SliceInString(EnvBlacklistFilter, strings.ToLower(es[0])) {
				continue
			}
			EnvVars = append(EnvVars, corev1.EnvVar{Name: es[0], Value: es[1]})
		}
	}

	// Resource labels
	l := Labels(objname, labelsFromParams(params.Labels))

	km := Variables.KeyMap()
	for _, e := range mg.Spec.Environment.Build {
		if e.Required == true {
			if len(km[e.Name]) > 0 {
				EnvVars = append(EnvVars, corev1.EnvVar{Name: e.Name, Value: km[e.Name]})
			} else {
				EnvVars = append(EnvVars, corev1.EnvVar{Name: e.Name, Value: e.Default})
			}
		} else if e.Required == false {
			EnvVars = append(EnvVars, corev1.EnvVar{Name: e.Name, Value: "null"})
		}
	}

	// Construct toObjRef for BuildConfig output overrides
	var toObjRefName = objname
	var toObjRefTag = "latest"
	if len(params.OutputImagestream) > 0 {
		toObjRefName = params.OutputImagestream
	}
	if len(Tag) > 0 {
		toObjRefTag = Tag
	}
	var toObjRef = &corev1.ObjectReference{
		Kind: "ImageStreamTag",
		Name: toObjRefName + ":" + toObjRefTag,
	}

	bc := buildv1.BuildConfig{
		TypeMeta: metav1.TypeMeta{
			Kind:       "BuildConfig",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   objname,
			Labels: l,
		},
		Spec: buildv1.BuildConfigSpec{
			RunPolicy: buildv1.BuildRunPolicySerial,
			CommonSpec: buildv1.CommonSpec{
				Source: buildsource,
				Strategy: buildv1.BuildStrategy{
					Type: buildv1.SourceBuildStrategyType,
					SourceStrategy: &buildv1.SourceBuildStrategy{
						Env: EnvVars,
						From: corev1.ObjectReference{
							Kind:      "ImageStreamTag",
							Namespace: imgurl.Namespace,
							Name:      imgurl.Image + ":" + imgurl.Tag,
						},
					},
				},
				Output: buildv1.BuildOutput{
					To: toObjRef,
				},
			},
		},
	}

	if !Dryrun {
		StoreBuildConfig(bc)
	}
	if Output {
		MarshalObject(bc.DeepCopyObject())
	}
}

func genBinaryBuildSource() buildv1.BuildSource {
	return buildv1.BuildSource{
		Type:   "Source",
		Binary: &buildv1.BinaryBuildSource{},
	}
}

func genGitBuildSource(mg *metagraf.MetaGraf) buildv1.BuildSource {
	var branch string
	if len(params.SourceRef) > 0 {
		branch = params.SourceRef
	} else {
		branch = mg.Spec.Branch
	}

	bs := buildv1.BuildSource{
		Type: "Git",
		Git: &buildv1.GitBuildSource{
			URI: mg.Spec.Repository,
			Ref: branch,
		},
	}

	if len(mg.Spec.RepSecRef) > 0 {
		bs.SourceSecret = &corev1.LocalObjectReference{
			Name: mg.Spec.RepSecRef,
		}
	}

	return bs
}

func StoreBuildConfig(obj buildv1.BuildConfig) {
	client := k8sclient.GetBuildClient().BuildConfigs(NameSpace)
	bc, _ := client.Get(context.TODO(), obj.Name, metav1.GetOptions{})

	if len(bc.ResourceVersion) > 0 {
		obj.ResourceVersion = bc.ResourceVersion
		_, err := client.Update(context.TODO(), &obj, metav1.UpdateOptions{})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Updated BuildConfig: ", obj.Name, " in Namespace: ", NameSpace)
	} else {
		_, err := client.Create(context.TODO(), &obj, metav1.CreateOptions{})
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
		fmt.Println("Created BuildConfig: ", obj.Name, " in Namespace: ", NameSpace)
	}
}

func DeleteBuildConfig(name string) {
	client := k8sclient.GetBuildClient().BuildConfigs(NameSpace)

	_, err := client.Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		fmt.Println("The BuildConfig: ", name, "does not exist in namespace: ", NameSpace, ", skipping...")
		return
	}

	err = client.Delete(context.TODO(), name, metav1.DeleteOptions{})
	if err != nil {
		fmt.Println("Unable to delete BuildConfig: ", name, " in namespace: ", NameSpace)
		log.Error(err)
		return
	}
	fmt.Println("Deleted BuildConfig: ", name, ", in namespace: ", NameSpace)
}
