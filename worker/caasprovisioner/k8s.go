// Copyright 2017 Canonical Ltd.
// Licensed under the AGPLv3, see LICENCE file for details.

package caasprovisioner

import (
	"github.com/juju/errors"
	"k8s.io/client-go/kubernetes"
	k8serrors "k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"
	"k8s.io/client-go/rest"

	"github.com/juju/juju/state"
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

func ensureOperator(client *kubernetes.Clientset, appName string) error {
	if exists, err := operatorExists(client, appName); err != nil {
		return errors.Trace(err)
	} else if exists {
		logger.Infof("%s operator already deployed", appName)
		return nil
	}
	logger.Infof("deploying %s operator", appName)
	return deployOperator(client, appName)
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

func deployOperator(client *kubernetes.Clientset, appName string) error {
	spec := &v1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name: podName(appName),
		},
		Spec: v1.PodSpec{
			// XXX Provide Juju API config details to the operator container
			Containers: []v1.Container{{
				Name:            "juju-operator",
				Image:           "mikemccracken/caasoperator",
				ImagePullPolicy: v1.PullAlways,
			}},
		},
	}
	_, err := client.CoreV1().Pods(namespace).Create(spec)
	return errors.Trace(err)
}

func podName(appName string) string {
	return "juju-operator-" + appName
}
