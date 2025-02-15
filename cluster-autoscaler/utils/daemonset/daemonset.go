/*
Copyright 2017 The Kubernetes Authors.

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

package daemonset

import (
	"fmt"
	"math/rand"

	"k8s.io/autoscaler/cluster-autoscaler/simulator"

	appsv1 "k8s.io/api/apps/v1"
	apiv1 "k8s.io/api/core/v1"
	schedulernodeinfo "k8s.io/kubernetes/pkg/scheduler/nodeinfo"
)

// GetDaemonSetPodsForNode returns daemonset nodes for the given pod.
func GetDaemonSetPodsForNode(nodeInfo *schedulernodeinfo.NodeInfo, daemonsets []*appsv1.DaemonSet, predicateChecker simulator.PredicateChecker) ([]*apiv1.Pod, error) {
	result := make([]*apiv1.Pod, 0)

	// here we can use empty snapshot
	clusterSnapshot := simulator.NewBasicClusterSnapshot()

	// add a node with pods - node info is created by cloud provider,
	// we don't know whether it'll have pods or not.
	if err := clusterSnapshot.AddNodeWithPods(nodeInfo.Node(), nodeInfo.Pods()); err != nil {
		return nil, err
	}

	for _, ds := range daemonsets {
		pod := newPod(ds, nodeInfo.Node().Name)
		err := predicateChecker.CheckPredicates(clusterSnapshot, pod, nodeInfo.Node().Name)
		if err == nil {
			result = append(result, pod)
		} else if err.ErrorType() == simulator.NotSchedulablePredicateError {
			// ok; we are just skipping this daemonset
		} else {
			// unexpected error
			return nil, fmt.Errorf("unexpected error while calling PredicateChecker; %v", err)
		}
	}
	return result, nil
}

// If only the limits are specified, requests are automatically set to match the limits. Since the scheduler doesn't
// do this, fixup the requests instead.
func fixupContainers(spec *apiv1.PodSpec) *apiv1.PodSpec {
	newSpec := spec.DeepCopy()
	for i := range newSpec.Containers {
		container := newSpec.Containers[i]
		for resourceName, resourceValue := range container.Resources.Limits {
			if _, ok := container.Resources.Requests[resourceName]; !ok {
				if container.Resources.Requests == nil {
					container.Resources.Requests = make(apiv1.ResourceList)
				}
				container.Resources.Requests[resourceName] = resourceValue
			}
		}
		newSpec.Containers[i] = container
	}
	return newSpec
}

func newPod(ds *appsv1.DaemonSet, nodeName string) *apiv1.Pod {
	podSpec := fixupContainers(&ds.Spec.Template.Spec)
	newPod := &apiv1.Pod{Spec: *podSpec, ObjectMeta: ds.Spec.Template.ObjectMeta}
	newPod.Namespace = ds.Namespace
	newPod.Name = fmt.Sprintf("%s-pod-%d", ds.Name, rand.Int63())
	newPod.Spec.NodeName = nodeName
	return newPod
}
