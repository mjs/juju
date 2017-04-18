// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasprovisioner

import (
	"github.com/juju/errors"
	"gopkg.in/juju/names.v2"
	"k8s.io/client-go/kubernetes"
	k8serrors "k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"

	"github.com/juju/juju/agent"
	"github.com/juju/juju/network"
	"github.com/juju/juju/state"
	"github.com/juju/juju/version"
)

// XXX should be using a juju specific namespace
const namespace = "default"

func newK8sClient(st *state.CAASState) (*kubernetes.Clientset, error) {
	model, err := st.CAASModel()
	if err != nil {
		return nil, errors.Trace(err)
	}
	config := newK8sConfig(model)
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, errors.Trace(err)
	}
	return client, nil
}

func newK8sConfig(model *state.CAASModel) *rest.Config {
	return &rest.Config{
		Host: model.Endpoint(),
		TLSClientConfig: rest.TLSClientConfig{
			CertData: model.CertData(),
			KeyData:  model.KeyData(),
			CAData:   model.CAData(),
		},
	}
}

func ensureOperator(client *kubernetes.Clientset, appName string, st *state.CAASState) error {
	if exists, err := operatorExists(client, appName); err != nil {
		return errors.Trace(err)
	} else if exists {
		logger.Infof("%s operator already deployed", appName)
		return nil
	}
	logger.Infof("deploying %s operator", appName)

	config, err := newOperatorConfig(appName, st)
	if err != nil {
		return errors.Trace(err)
	}
	return deployOperator(client, appName, config)
}

func operatorExists(client *kubernetes.Clientset, appName string) (bool, error) {
	_, err := client.CoreV1().Pods(namespace).Get(podName(appName))
	if k8serrors.IsNotFound(err) {
		return false, nil
	} else if err != nil {
		return false, errors.Trace(err)
	}
	return true, nil
}

func deployOperator(client *kubernetes.Clientset, appName string, config []byte) error {
	configMapName := podName(appName) + "-config"
	configVolName := configMapName + "-volume"

	_, err := client.ConfigMaps(namespace).Create(&v1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Name: configMapName,
		},
		Data: map[string]string{
			"agent.conf": string(config),
		},
	})
	if err != nil {
		return errors.Trace(err)
	}

	appTag := names.NewApplicationTag(appName)
	spec := &v1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: podName(appName),
		},
		Spec: v1.PodSpec{
			Containers: []v1.Container{{
				Name:            "juju-operator",
				Image:           "mikemccracken/caasoperator",
				ImagePullPolicy: v1.PullAlways,
				Args:            []string{"caasoperator", "--application-name", appName},
				VolumeMounts: []v1.VolumeMount{{
					Name: configVolName,
					// XXX shouldn't be hardcoded
					MountPath: "/var/lib/juju/agents/" + appTag.String(),
				}},
			}},
			Volumes: []v1.Volume{{
				Name: configVolName,
				VolumeSource: v1.VolumeSource{
					ConfigMap: &v1.ConfigMapVolumeSource{
						LocalObjectReference: v1.LocalObjectReference{
							Name: configMapName,
						},
					},
				},
			}},
		},
	}
	_, err = client.CoreV1().Pods(namespace).Create(spec)
	return errors.Trace(err)
}

func podName(appName string) string {
	return "juju-operator-" + appName
}

func newOperatorConfig(appName string, st *state.CAASState) ([]byte, error) {
	appTag := names.NewApplicationTag(appName)

	apiAddrs, err := apiAddresses(st)
	if err != nil {
		return nil, errors.Trace(err)
	}

	controllerCfg, err := st.ControllerConfig()
	if err != nil {
		return nil, errors.Trace(err)
	}
	caCert, ok := controllerCfg.CACert()
	if !ok {
		return nil, errors.New("missing ca cert in controller config")
	}

	conf, err := agent.NewAgentConfig(
		agent.AgentConfigParams{
			Paths: agent.Paths{
				// XXX shouldn't be hardcoded
				DataDir: "/var/lib/juju",
				LogDir:  "/var/log/juju",
			},
			// This isn't actually used but needs to be supplied.
			UpgradedToVersion: version.Current,
			Tag:               appTag,
			Password:          "XXX not currently checked",
			Controller:        st.ControllerTag(),
			Model:             st.ModelTag(),
			APIAddresses:      apiAddrs,
			CACert:            caCert,
		},
	)
	if err != nil {
		return nil, errors.Trace(err)
	}

	confBytes, err := conf.Render()
	if err != nil {
		return nil, errors.Trace(err)
	}
	return confBytes, nil
}

func apiAddresses(st *state.CAASState) ([]string, error) {
	apiHostPorts, err := st.APIHostPorts()
	if err != nil {
		return nil, err
	}
	var addrs = make([]string, 0, len(apiHostPorts))
	for _, hostPorts := range apiHostPorts {
		ordered := network.PrioritizeInternalHostPorts(hostPorts, false)
		for _, addr := range ordered {
			if addr != "" {
				addrs = append(addrs, addr)
			}
		}
	}
	return addrs, nil
}
