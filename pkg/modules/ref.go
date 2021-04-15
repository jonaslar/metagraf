/*
Copyright 2019-2020 The metaGraf Authors

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
	"fmt"
	"github.com/laetho/metagraf/internal/pkg/params"
	"github.com/laetho/metagraf/pkg/metagraf"
	"html/template"
	"io/ioutil"
	log "k8s.io/klog"
	"k8s.io/klog/v2"
	"os"
	"strings"
)

// Returns a metagraf.MGProperties map of parameters mainly for use in
// template processing. It filters out JVM_SYS_PROP type.
func getPropSlice(mg *metagraf.MetaGraf) []metagraf.MGProperty {
	var props []metagraf.MGProperty

	// No defaults for Environment Variables.
	for _, e := range mg.Spec.Environment.Local {
		if e.Type == "JVM_SYS_PROP" {
			continue
		}
		p := metagraf.MGProperty{
			Source:   "local",
			Key:      e.Name,
			Value:    "",
			Required: e.Required,
			Default:  "",
		}
		props = append(props, p)
	}

	for _, c := range mg.Spec.Config {
		if c.Name != "JVM_SYS_PROP" && c.Name != "jvm.options" {
			continue
		}
		for _, o := range c.Options {
			p := metagraf.MGProperty{
				Source:   c.Name,
				Key:      o.Name,
				Value:    "",
				Required: o.Required,
				Default:  "",
			}
			if c.Name == "JVM_SYS_PROP" {
				p.Default = o.Default
			}
			props = append(props, p)
		}
	}
	return props
}

// Returns a output sanitized slice of metagraf.EnvironmentVar{} for templates
func getEnvsForTemplate(mg *metagraf.MetaGraf, nojsp bool) []metagraf.EnvironmentVar {
	var envs []metagraf.EnvironmentVar

	for _, e := range mg.Spec.Environment.Local {
		// Skip JVM_SYS_PROP type if nojsp = true
		if nojsp && e.Type == "JVM_SYS_PROP" {
			continue
		}
		me := metagraf.EnvironmentVar{
			Name:        e.Name,
			Required:    e.Required,
			Type:        e.Type,
			EnvFrom:     e.EnvFrom,
			SecretFrom:  e.SecretFrom,
			Description: e.Description,
			Default:     e.Default,
			Example:     e.Example,
		}
		if len(e.SecretFrom) > 0 {
			me.Name = me.SecretFrom
			me.Type = "***Secret***"
			me.Description = "A referenced secret. See secret section."
		}
		if len(e.EnvFrom) > 0 {
			me.Name = me.EnvFrom
			me.Type = "***EnvFrom***"
			me.Description = "Environment variables from file. See config section for details."
		}
		envs = append(envs, me)
	}
	return envs
}

// Filters out only required property elements.
func getReqPropSlice(props []metagraf.MGProperty) []metagraf.MGProperty {
	var mgprops []metagraf.MGProperty
	for _, p := range props {
		if p.Required {
			mgprops = append(mgprops, p)
		}
	}
	return mgprops
}

func GenRef(mg *metagraf.MetaGraf) {
	// Template string
	ts := ""

	if len(params.RefTemplateFile) > 0 {
		_, err := os.Stat(params.RefTemplateFile)
		if os.IsNotExist(err) {
			log.Infof("Fetching template: %v", Template)
			cm, err := GetConfigMap(Template)
			if err != nil {
				log.Error(err)
				os.Exit(-1)
			}
			ts = cm.Data["template"]
		} else {
			log.Infof("Fetching template from file: %v", params.RefTemplateFile)
			c, err := ioutil.ReadFile(params.RefTemplateFile)
			if err != nil {
				panic(err)
			}
			ts = string(c)
		}

	}

	tmpl, err := template.New("refdoc").Funcs(
		template.FuncMap{
			"getPropSlice": func() []metagraf.MGProperty {
				return getPropSlice(mg)
			},
			"getReqPropSlice": func() []metagraf.MGProperty {
				return getReqPropSlice(getPropSlice(mg))
			},
			"lenPropMap": func(p []metagraf.MGProperty) int {
				return len(p)
			},
			"getEnvsForTemplate": func(f bool) []metagraf.EnvironmentVar {
				return getEnvsForTemplate(mg, f)
			},
			"split": func(s string, d string) []string {
				return strings.Split(s, d)
			},
			"numOfLocal": func(l []metagraf.EnvironmentVar) int {
				return len(l)
			},
			"numOfOptions": func(l []metagraf.ConfigParam) int {
				return len(l)
			},
			"isLast": func(a []string, k int) bool {
				if len(a)-1 == k {
					return true
				}
				return false
			},
			"last": func(t int, c int) bool {
				if t-1 == c {
					return true
				}
				return false
			},
		}).Parse(ts)
	if err != nil {
		log.Error(err)
		os.Exit(1)
	}

	filename := ""
	if len(params.RefTemplateOutputFile) > 0 {
		filename = params.RefTemplateOutputFile
	} else {
		filename = "/tmp/" + Name(mg) + Suffix
	}

	f, err := os.OpenFile(filename, os.O_CREATE|os.O_TRUNC|os.O_RDWR, 0777)
	if err != nil {
		fmt.Println(err)
		return
	}
	defer f.Close()

	klog.Infof("Build envs: %v", len(mg.Spec.Environment.Build))
	err = tmpl.Execute(f, mg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	fmt.Println("Wrote ref file to: ", filename)
}
