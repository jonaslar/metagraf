/*
Copyright 2018 The metaGraf Authors

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

package cmd

import (
	"fmt"
	"github.com/laetho/metagraf/internal/pkg/params"
	"github.com/laetho/metagraf/pkg/metagraf"
	"github.com/laetho/metagraf/pkg/modules"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	log "k8s.io/klog"

	"os"
)

func init() {
	RootCmd.AddCommand(devCmd)
	devCmd.AddCommand(devCmdUp)
	devCmd.AddCommand(devCmdDown)
	devCmdUp.Flags().StringVarP(&Namespace, "namespace", "n", "", "namespace to work on, if not supplied it will use current active namespace.")
	devCmdUp.Flags().StringVar(&params.SourceRef, "ref", "", "use for overriding source ref or branch ref in buildconfig.")
	devCmdUp.Flags().StringSliceVar(&CVars, "cvars", []string{}, "Slice of key=value pairs, seperated by ,")
	devCmdUp.Flags().StringVar(&params.PropertiesFile, "cvfile", "", "Property file with component configuration values. Can be generated with \"mg generate properties\" command.)")
	devCmdUp.Flags().StringVar(&OName, "name", "", "Overrides name of application.")
	devCmdUp.Flags().StringVarP(&Registry, "registry", "r", viper.GetString("registry"), "Specify container registry host")
	devCmdUp.Flags().StringVarP(&params.OutputImagestream, "istream", "i", "", "specify if you want to output to another imagestream than the component name")
	devCmdUp.Flags().StringVarP(&Context, "context", "c", "/", "Application contextroot. (\"/<context>\"). Used when creating Route object.")
	devCmdUp.Flags().BoolVarP(&CreateGlobals, "globals", "g", false, "Override default behavior and force creation of global secrets. Will not overwrite existing ones.")
	devCmdUp.Flags().BoolVar(&params.ServiceMonitor, "service-monitor", false, "Set flag to also create a ServiceMonitor resource. Requires a cluster with the prometheus-operator.")
	devCmdUp.Flags().Int32Var(&params.ServiceMonitorPort, "service-monitor-port", params.ServiceMonitorPort, "Set Service port to scrape in ServiceMonitor.")
	devCmdUp.Flags().StringVar(&params.ServiceMonitorOperatorName, "service-monitor-operator-name", params.ServiceMonitorOperatorName, "Name of prometheus-operator instance to create ServiceMonitor for.")
	devCmdDown.Flags().StringVarP(&Namespace, "namespace", "n", "", "namespace to work on, if not supplied it will use current active namespace.")
	devCmdDown.Flags().BoolVar(&params.Everything, "everything", false, "Delete all resources and artifacts generated from mg dev up.")
	devCmdDown.Flags().StringVar(&OName, "name", "", "Overrides name of application.")
}

var devCmd = &cobra.Command{
	Use:   "dev",
	Short: "dev subcommands",
	Long:  `dev subcommands`,
}

var devCmdUp = &cobra.Command{
	Use:   "up <metagraf.json>",
	Short: "creates the required component resources.",
	Long:  `sets up the required resources to test the component in the platform.`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			log.Info(StrActiveProject, viper.Get("namespace"))
			fmt.Println(StrMissingMetaGraf)
			os.Exit(1)
		}

		if len(Namespace) == 0 {
			Namespace = viper.GetString("namespace")
			if len(Namespace) == 0 {
				log.Error(StrMissingNamespace)
				os.Exit(1)
			}
		}
		FlagPassingHack()

		devUp(args[0])
	},
}

var devCmdDown = &cobra.Command{
	Use:   "down <metagraf.json>",
	Short: "deletes component resources",
	Long:  `dev subcommands`,
	Run: func(cmd *cobra.Command, args []string) {
		if len(args) < 1 {
			log.Info(StrActiveProject, viper.Get("namespace"))
			log.Error(StrMissingMetaGraf)
			return
		}

		if len(Namespace) == 0 {
			Namespace = viper.GetString("namespace")
			if len(Namespace) == 0 {
				log.Error(StrMissingNamespace)
				os.Exit(1)
			}
		}
		FlagPassingHack()

		devDown(args[0])
	},
}

func devUp(mgf string) {
	mg := metagraf.Parse(mgf)
	modules.Variables = GetCmdProperties(mg.GetProperties())
	log.V(2).Info("Current MGProperties: ", modules.Variables)

	modules.GenSecrets(&mg)
	modules.GenConfigMaps(&mg)
	modules.GenImageStream(&mg, Namespace)
	modules.GenBuildConfig(&mg)
	modules.GenDeploymentConfig(&mg)
	modules.GenService(&mg)
	modules.GenRoute(&mg)

}

func devDown(mgf string) {
	mg := metagraf.Parse(mgf)
	basename := modules.Name(&mg)

	modules.DeleteRoute(basename)
	modules.DeleteService(basename)
	modules.DeleteServiceMonitor(basename)
	modules.DeleteDeploymentConfig(basename)
	modules.DeleteBuildConfig(basename)
	modules.DeleteConfigMaps(&mg)
	modules.DeleteImageStream(basename)

	if params.Everything {
		modules.DeleteSecrets(&mg)
	}
}
