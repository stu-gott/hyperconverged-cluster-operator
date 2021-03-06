/*
Copyright 2018 The CDI Authors.

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

package namespaced

import (
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"

	"kubevirt.io/containerized-data-importer/pkg/common"
	"kubevirt.io/containerized-data-importer/pkg/controller"
	utils "kubevirt.io/containerized-data-importer/pkg/operator/resources/utils"
)

const (
	controllerServiceAccount = "cdi-sa"
	prometheusLabel          = common.PrometheusLabel
	privilegedAccountPrefix  = "system:serviceaccount"
)

func getControllerPrivilegedAccounts(args *FactoryArgs) []string {
	return []string{
		fmt.Sprintf("%s:%s:%s", privilegedAccountPrefix, args.Namespace, controllerServiceAccount),
	}
}

func createControllerResources(args *FactoryArgs) []runtime.Object {
	return []runtime.Object{
		createControllerServiceAccount(),
		createControllerDeployment(args.DockerRepo,
			args.ControllerImage,
			args.ImporterImage,
			args.ClonerImage,
			args.UploadServerImage,
			args.DockerTag,
			args.Verbosity,
			args.PullPolicy),
		createInsecureRegConfigMap(),
	}
}

func createControllerServiceAccount() *corev1.ServiceAccount {
	return utils.CreateServiceAccount(controllerServiceAccount)
}

func createControllerDeployment(repo, controllerImage, importerImage, clonerImage, uploadServerImage, tag, verbosity, pullPolicy string) *appsv1.Deployment {
	deployment := utils.CreateDeployment("cdi-deployment", "app", "containerized-data-importer", controllerServiceAccount, int32(1))
	container := utils.CreateContainer("cdi-controller", repo, controllerImage, tag, verbosity, corev1.PullPolicy(pullPolicy))
	container.Env = []corev1.EnvVar{
		{
			Name:  "IMPORTER_IMAGE",
			Value: fmt.Sprintf("%s/%s:%s", repo, importerImage, tag),
		},
		{
			Name:  "CLONER_IMAGE",
			Value: fmt.Sprintf("%s/%s:%s", repo, clonerImage, tag),
		},
		{
			Name:  "UPLOADSERVER_IMAGE",
			Value: fmt.Sprintf("%s/%s:%s", repo, uploadServerImage, tag),
		},
		{
			Name:  "UPLOADPROXY_SERVICE",
			Value: uploadProxyResourceName,
		},
		{
			Name:  "PULL_POLICY",
			Value: pullPolicy,
		},
	}
	container.ReadinessProbe = &corev1.Probe{
		Handler: corev1.Handler{
			Exec: &corev1.ExecAction{
				Command: []string{"cat", "/tmp/ready"},
			},
		},
		InitialDelaySeconds: 2,
		PeriodSeconds:       5,
	}
	container.VolumeMounts = []corev1.VolumeMount{
		{
			Name:      "cdi-api-signing-key",
			MountPath: controller.APIServerPublicKeyDir,
		},
	}
	deployment.Spec.Template.Spec.Volumes = []corev1.Volume{
		{
			Name: "cdi-api-signing-key",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: "cdi-api-signing-key",
					Items: []corev1.KeyToPath{
						{
							Key:  "id_rsa.pub",
							Path: "id_rsa.pub",
						},
					},
				},
			},
		},
	}
	deployment.Spec.Template.Spec.Containers = []corev1.Container{container}
	return deployment
}

func createPrometheusService() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name: "kubevirt-prometheus-metrics",
			Labels: map[string]string{
				prometheusLabel: "",
				"kubevirt.io":   "",
			},
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				prometheusLabel: "",
			},
			Ports: []corev1.ServicePort{
				{
					Name: "metrics",
					Port: 443,
					TargetPort: intstr.IntOrString{
						Type:   intstr.String,
						StrVal: "metrics",
					},
					Protocol: corev1.ProtocolTCP,
				},
			},
		},
	}
}

func createInsecureRegConfigMap() *corev1.ConfigMap {
	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:   common.InsecureRegistryConfigMap,
			Labels: utils.WithCommonLabels(nil),
		},
	}
}
