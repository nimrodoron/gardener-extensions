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

package generator

import (
	"github.com/coreos/go-systemd/unit"
	"github.com/gardener/gardener-extensions/pkg/controller/operatingsystemconfig/oscommon/generator"
	"github.com/gardener/gardener-extensions/pkg/webhook/controlplane"
	"text/template"

	extensionswebhook "github.com/gardener/gardener-extensions/pkg/webhook"
	v1alpha1constants "github.com/gardener/gardener/pkg/apis/core/v1alpha1/constants"

	utils "github.com/gardener/gardener-extensions/pkg/controller/operatingsystemconfig"
	template_gen "github.com/gardener/gardener-extensions/pkg/controller/operatingsystemconfig/oscommon/template"
	"github.com/gobuffalo/packr/v2"
	"k8s.io/apimachinery/pkg/util/runtime"
)

var cmd = "/usr/bin/cloud-init clean && /usr/bin/cloud-init --file %s init"
var cloudInitGenerator *UbuntuCloudInitGenerator

//go:generate packr2

func init() {
	box := packr.New("templates", "./templates")
	cloudInitTemplateString, err := box.FindString("cloud-init-ubuntu.template")
	runtime.Must(err)

	cloudInitTemplate, err := template.New("cloud-init").Parse(cloudInitTemplateString)
	runtime.Must(err)
	cloudInitGenerator = new(UbuntuCloudInitGenerator)
	cloudInitGenerator.gen = template_gen.NewCloudInitGenerator(cloudInitTemplate, template_gen.DefaultUnitsPath, cmd)
	cloudInitGenerator.unitSerializer = controlplane.NewUnitSerializer()
}

// CloudInitGenerator is the generator which will genereta the cloud init yaml
func CloudInitGenerator() *UbuntuCloudInitGenerator {
	return cloudInitGenerator
}

type UbuntuCloudInitGenerator struct {
	gen            *template_gen.CloudInitGenerator
	unitSerializer controlplane.UnitSerializer
}

func (t *UbuntuCloudInitGenerator) Generate(data *generator.OperatingSystemConfig) ([]byte, *string, error) {
	var opts []*unit.UnitOption
	var err error

	if u := utils.UnitOptionWithName(data.Units, v1alpha1constants.OperatingSystemConfigUnitNameKubeletService); u != nil {
		// Deserialize unit options
		content := string(u.Content)
		if opts, err = t.unitSerializer.Deserialize(content); err != nil {
			//return errors.Wrap(err, "could not deserialize kubelet.service unit content")
		}
		if opt := extensionswebhook.UnitOptionWithSectionAndName(opts, "Service", "ExecStart"); opt != nil {
			command := extensionswebhook.DeserializeCommandLine(opt.Value)
			command = ensureKubeletCommandLineArgs(command)
			opt.Value = extensionswebhook.SerializeCommandLine(command, 1, " \\\n    ")
		}
		// Serialize unit options
		if content, err = t.unitSerializer.Serialize(opts); err != nil {
			//return errors.Wrap(err, "could not serialize kubelet.service unit options")
		}
		u.Content = []byte(content)

	}
	return t.gen.Generate(data)
}

func ensureKubeletCommandLineArgs(command []string) []string {
	command = extensionswebhook.EnsureStringWithPrefix(command, "--container-runtime=", "remote")
	command = extensionswebhook.EnsureStringWithPrefix(command, "--container-runtime-endpoint=", "unix:///run/containerd/containerd.sock")
	return command
}
