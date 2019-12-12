// Copyright (c) 2019 SAP SE or an SAP affiliate company. All rights reserved. This file is licensed under the Apache Software License, v. 2 except as noted otherwise in the LICENSE file
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"context"
	"os"

	"github.com/gardener/gardener-extensions/pkg/controller/operatingsystemconfig/oscommon"
	"github.com/gardener/gardener-extensions/pkg/util"
	"sigs.k8s.io/controller-runtime/pkg/manager"

	componentbaseconfig "k8s.io/component-base/config"

	"github.com/gardener/gardener-extensions/controllers/os-ubuntu/pkg/generator"
	controlplanewebhook "github.com/gardener/gardener-extensions/controllers/os-ubuntu/pkg/webhook/controlplane"
	extcontroller "github.com/gardener/gardener-extensions/pkg/controller"
	controllercmd "github.com/gardener/gardener-extensions/pkg/controller/cmd"
	oscommoncmd "github.com/gardener/gardener-extensions/pkg/controller/operatingsystemconfig/oscommon/cmd"
	webhookcmd "github.com/gardener/gardener-extensions/pkg/webhook/cmd"
	extensioncontrolplanewebhook "github.com/gardener/gardener-extensions/pkg/webhook/controlplane"

	"github.com/spf13/cobra"
)

// NewControllerCommand returns a new Command with a new Generator
func NewControllerCommand(ctx context.Context) *cobra.Command {

	g := generator.CloudInitGenerator()
	if g == nil {
		controllercmd.LogErrAndExit(nil, "Could not create Generator")
	}

	var (
		osName   = "ubuntu"
		restOpts = &controllercmd.RESTOptions{}
		mgrOpts  = &controllercmd.ManagerOptions{
			LeaderElection:          true,
			LeaderElectionID:        controllercmd.LeaderElectionNameID(osName),
			LeaderElectionNamespace: os.Getenv("LEADER_ELECTION_NAMESPACE"),
			WebhookServerPort:       443,
		}
		ctrlOpts = &controllercmd.ControllerOptions{
			MaxConcurrentReconciles: 5,
		}

		reconcileOpts = &controllercmd.ReconcilerOptions{}

		// options for the webhook server
		webhookServerOptions = &webhookcmd.ServerOptions{
			CertDir:   "/tmp/gardener-extensions-cert",
			Namespace: os.Getenv("WEBHOOK_CONFIG_NAMESPACE"),
		}

		controllerSwitches = oscommoncmd.SwitchOptions(osName, g)
		webhookSwitches    = webhookcmd.NewSwitchOptions(webhookcmd.Switch(extensioncontrolplanewebhook.WebhookName, controlplanewebhook.AddToManager))
		webhookOptions     = webhookcmd.NewAddToManagerOptions("os-ubuntu", webhookServerOptions, webhookSwitches)

		aggOption = controllercmd.NewOptionAggregator(
			restOpts,
			mgrOpts,
			ctrlOpts,
			reconcileOpts,
			controllerSwitches,
			webhookOptions,
		)
	)

	cmd := &cobra.Command{
		Use: "os-" + osName + "-controller-manager",

		Run: func(cmd *cobra.Command, args []string) {
			if err := aggOption.Complete(); err != nil {
				controllercmd.LogErrAndExit(err, "Error completing options")
			}

			// TODO: Make these flags configurable via command line parameters or component config file.
			util.ApplyClientConnectionConfigurationToRESTConfig(&componentbaseconfig.ClientConnectionConfiguration{
				QPS:   100.0,
				Burst: 130,
			}, restOpts.Completed().Config)

			mgr, err := manager.New(restOpts.Completed().Config, mgrOpts.Completed().Options())
			if err != nil {
				controllercmd.LogErrAndExit(err, "Could not instantiate manager")
			}

			if err := extcontroller.AddToScheme(mgr.GetScheme()); err != nil {
				controllercmd.LogErrAndExit(err, "Could not update manager scheme")
			}

			ctrlOpts.Completed().Apply(&oscommon.DefaultAddOptions.Controller)

			reconcileOpts.Completed().Apply(&oscommon.DefaultAddOptions.IgnoreOperationAnnotation)

			//_, shootWebhooks, err :=
			webhookOptions.Completed().AddToManager(mgr)
			/*			if err != nil {
						controllercmd.LogErrAndExit(err, "Could not add webhooks to manager")
					}*/
			//awscontrolplane.DefaultAddOptions.ShootWebhooks = shootWebhooks
			//oscommon.DefaultAddOptions.
			if err := controllerSwitches.Completed().AddToManager(mgr); err != nil {
				controllercmd.LogErrAndExit(err, "Could not add controller to manager")
			}

			if err := mgr.Start(ctx.Done()); err != nil {
				controllercmd.LogErrAndExit(err, "Error running manager")
			}
		},
	}

	aggOption.AddFlags(cmd.Flags())

	return cmd
}
