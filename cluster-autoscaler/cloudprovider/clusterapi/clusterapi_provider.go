/*
Copyright 2020 The Kubernetes Authors.

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

package clusterapi

import (
	"reflect"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/autoscaler/cluster-autoscaler/cloudprovider"
	"k8s.io/autoscaler/cluster-autoscaler/config"
	"k8s.io/autoscaler/cluster-autoscaler/utils/errors"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog"
)

const (
	// ProviderName is the name of cluster-api cloud provider.
	ProviderName = "clusterapi"

	// GPULabel is the label added to nodes with GPU resource.
	GPULabel = "cluster-api/accelerator"
)

var _ cloudprovider.CloudProvider = (*provider)(nil)

type provider struct {
	controller      *machineController
	providerName    string
	resourceLimiter *cloudprovider.ResourceLimiter
}

func (p *provider) Name() string {
	return p.providerName
}

func (p *provider) GetResourceLimiter() (*cloudprovider.ResourceLimiter, error) {
	return p.resourceLimiter, nil
}

func (p *provider) NodeGroups() []cloudprovider.NodeGroup {
	var result []cloudprovider.NodeGroup
	nodegroups, err := p.controller.nodeGroups()
	if err != nil {
		klog.Errorf("error getting node groups: %v", err)
		return nil
	}
	for _, ng := range nodegroups {
		klog.V(4).Infof("discovered node group: %s", ng.Debug())
		result = append(result, ng)
	}
	return result
}

func (p *provider) NodeGroupForNode(node *corev1.Node) (cloudprovider.NodeGroup, error) {
	ng, err := p.controller.nodeGroupForNode(node)
	if err != nil {
		return nil, err
	}
	if ng == nil || reflect.ValueOf(ng).IsNil() {
		return nil, nil
	}
	return ng, nil
}

func (*provider) Pricing() (cloudprovider.PricingModel, errors.AutoscalerError) {
	return nil, cloudprovider.ErrNotImplemented
}

func (*provider) GetAvailableMachineTypes() ([]string, error) {
	return []string{}, nil
}

func (*provider) NewNodeGroup(
	machineType string,
	labels map[string]string,
	systemLabels map[string]string,
	taints []corev1.Taint,
	extraResources map[string]resource.Quantity,
) (cloudprovider.NodeGroup, error) {
	return nil, cloudprovider.ErrNotImplemented
}

func (*provider) Cleanup() error {
	return nil
}

func (p *provider) Refresh(existingNodes []*corev1.Node) error {
	return nil
}

// GetInstanceID gets the instance ID for the specified node.
func (p *provider) GetInstanceID(node *corev1.Node) string {
	return node.Spec.ProviderID
}

// GetAvailableGPUTypes return all available GPU types cloud provider supports.
func (p *provider) GetAvailableGPUTypes() map[string]struct{} {
	// TODO: implement this
	return nil
}

// GPULabel returns the label added to nodes with GPU resource.
func (p *provider) GPULabel() string {
	return GPULabel
}

func newProvider(
	name string,
	rl *cloudprovider.ResourceLimiter,
	controller *machineController,
) (cloudprovider.CloudProvider, error) {
	return &provider{
		providerName:    name,
		resourceLimiter: rl,
		controller:      controller,
	}, nil
}

// BuildClusterAPI builds CloudProvider implementation for machine api.
func BuildClusterAPI(opts config.AutoscalingOptions, do cloudprovider.NodeGroupDiscoveryOptions, rl *cloudprovider.ResourceLimiter) cloudprovider.CloudProvider {
	externalConfig, err := clientcmd.BuildConfigFromFlags("", opts.KubeConfigPath)
	if err != nil {
		klog.Fatalf("cannot build config: %v", err)
	}

	// Grab a dynamic interface that we can create informers from
	dc, err := dynamic.NewForConfig(externalConfig)
	if err != nil {
		klog.Fatalf("could not generate dynamic client for config")
	}

	kubeClient, err := kubernetes.NewForConfig(externalConfig)
	if err != nil {
		klog.Fatalf("create kube clientset failed: %v", err)
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(externalConfig)
	if err != nil {
		klog.Fatalf("create discovery client failed: %v", err)
	}

	controller, err := newMachineController(dc, kubeClient, discoveryClient)
	if err != nil {
		klog.Fatal(err)
	}

	// Ideally this would be passed in but the builder is not
	// currently organised to do so.
	stopCh := make(chan struct{})

	if err := controller.run(stopCh); err != nil {
		klog.Fatal(err)
	}

	provider, err := newProvider(ProviderName, rl, controller)
	if err != nil {
		klog.Fatal(err)
	}

	return provider
}
