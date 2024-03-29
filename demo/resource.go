package main

import (
	"fmt"
	v1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	netv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	typeappv1 "k8s.io/client-go/kubernetes/typed/apps/v1"
	typev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	typenetworkingv1 "k8s.io/client-go/kubernetes/typed/networking/v1"
)

const osDepPrefix = "os-dep-"
const ngxDepPrefix = "ngx-dep-"
const osSvcPrefix = "os-svc-"
const ngxCmPrefix = "ngx-conf-"
const ingressPathPrefix = "/osenv/"
const ingressName = "os-ingress"

var ingressClient typenetworkingv1.IngressInterface
var deploymentsClient typeappv1.DeploymentInterface
var svcClient typev1.ServiceInterface
var configMapClient typev1.ConfigMapInterface

var osDepT = &v1.Deployment{
	Spec: v1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: make(map[string]string),
		},
		Template: apiv1.PodTemplateSpec{
			Spec: apiv1.PodSpec{
				Containers: []apiv1.Container{
					{
						Name:            "os",
						Image:           "os:v3",
						ImagePullPolicy: "IfNotPresent",
						Ports: []apiv1.ContainerPort{
							{
								ContainerPort: 8080,
								Protocol:      "TCP",
								Name:          "codeserver",
							},
						},
						Env: []apiv1.EnvVar{
							{
								Name:  "CODE_SERVER_ARGS",
								Value: "--disable-workspace-trust --auth none",
							},
						},
					},
				},
			},
		},
	},
}

var ngxDepT = &v1.Deployment{
	Spec: v1.DeploymentSpec{
		Selector: &metav1.LabelSelector{
			MatchLabels: make(map[string]string),
		},
		Template: apiv1.PodTemplateSpec{
			Spec: apiv1.PodSpec{
				Volumes: []apiv1.Volume{
					{
						Name: "ngx-conf",
						VolumeSource: apiv1.VolumeSource{
							ConfigMap: &apiv1.ConfigMapVolumeSource{
								LocalObjectReference: apiv1.LocalObjectReference{
									Name: "",
								},
							},
						},
					},
				},
				Containers: []apiv1.Container{
					{
						Name:            "nginx",
						Image:           "nginx:alpine",
						ImagePullPolicy: "IfNotPresent",
						Ports: []apiv1.ContainerPort{
							{
								ContainerPort: 80,
								Protocol:      "TCP",
								Name:          "http",
							},
						},
						VolumeMounts: []apiv1.VolumeMount{
							{
								Name:      "ngx-conf",
								MountPath: "/etc/nginx/conf.d",
							},
						},
					},
				},
			},
		},
	},
}

var osSvcT = &apiv1.Service{
	Spec: apiv1.ServiceSpec{
		Selector: make(map[string]string),
		Ports: []apiv1.ServicePort{
			{
				Port: 4000,
				TargetPort: intstr.IntOrString{
					Type:   intstr.String,
					IntVal: 0,
					StrVal: "codeserver",
				},
				Protocol: "TCP",
				Name:     "code",
			},
			{
				Port: 80,
				TargetPort: intstr.IntOrString{
					Type:   intstr.String,
					IntVal: 0,
					StrVal: "http",
				},
				Protocol: "TCP",
				Name:     "nginx",
			},
		},
	},
}

var cmT = &apiv1.ConfigMap{
	Data: map[string]string{
		"default.conf": `upstream codeserver {
  server os-svc-%d:4000;
}
server {
  listen 80;
  location /osenv/%d/ {
        proxy_pass http://codeserver/;
      proxy_set_header Host $http_host;
      proxy_set_header Upgrade $http_upgrade;
      proxy_set_header Connection upgrade;
      proxy_set_header Accept-Encoding gzip;
    }

}`,
	},
}

var osIngressT = &netv1.Ingress{
	Spec: netv1.IngressSpec{
		Rules: []netv1.IngressRule{
			{
				IngressRuleValue: netv1.IngressRuleValue{
					HTTP: &netv1.HTTPIngressRuleValue{
						Paths: []netv1.HTTPIngressPath{},
					},
				},
			},
		},
	},
}

func init() {
	osIngressT.Name = ingressName
}

func newNgxConfigMap(envNum int) (*apiv1.ConfigMap, error) {

	newCm := cmT.DeepCopy()

	newCm.ObjectMeta.Name = fmt.Sprintf("%s%d", ngxCmPrefix, envNum)

	newCm.Data["default.conf"] = fmt.Sprintf(newCm.Data["default.conf"], envNum, envNum)

	return newCm, nil
}

func newOsSvc(envNum int) (*apiv1.Service, error) {
	newSvc := osSvcT.DeepCopy()

	newSvc.ObjectMeta.Name = fmt.Sprintf("%s%d", osSvcPrefix, envNum)

	newSvc.Spec.Selector["app"] = fmt.Sprintf("%s%d", osDepPrefix, envNum)

	return newSvc, nil
}

func newOsDep(envNum int) (*v1.Deployment, error) {
	newDep := osDepT.DeepCopy()

	var replicas int32 = 1
	newDep.Spec.Replicas = &replicas

	newDep.Spec.Selector.MatchLabels["app"] = fmt.Sprintf("%s%d", osDepPrefix, envNum)

	newDep.ObjectMeta.Name = fmt.Sprintf("%s%d", osDepPrefix, envNum)

	newDep.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	newDep.Spec.Template.ObjectMeta.Labels["app"] = fmt.Sprintf("%s%d", osDepPrefix, envNum)

	return newDep, nil
}

func newNgxDep(envNum int) (*v1.Deployment, error) {
	newDep := ngxDepT.DeepCopy()

	var replicas int32 = 1
	newDep.Spec.Replicas = &replicas

	newDep.Spec.Selector.MatchLabels["ngx"] = fmt.Sprintf("%s%d", ngxDepPrefix, envNum)

	newDep.ObjectMeta.Name = fmt.Sprintf("%s%d", ngxDepPrefix, envNum)

	newDep.Spec.Template.ObjectMeta.Labels = make(map[string]string)
	newDep.Spec.Template.ObjectMeta.Labels["app"] = fmt.Sprintf("%s%d", osDepPrefix, envNum)
	newDep.Spec.Template.ObjectMeta.Labels["ngx"] = fmt.Sprintf("%s%d", ngxDepPrefix, envNum)

	newDep.Spec.Template.Spec.Volumes[0].VolumeSource.ConfigMap.LocalObjectReference.Name = fmt.Sprintf("%s%d", ngxCmPrefix, envNum)

	return newDep, nil
}
