/*
Copyright 2016 The Kubernetes Authors.

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

package kubernetes

import (
	"fmt"
	"time"

	apiv1 "k8s.io/api/core/v1"
)

const (
	nodeNotReadyZalandoTaint = "zalando.org/node-not-ready"
)

// IsNodeReadyAndSchedulable returns true if the node is ready and schedulable.
func IsNodeReadyAndSchedulable(node *apiv1.Node) bool {
	ready, _, _ := GetReadinessState(node)
	if !ready {
		return false
	}
	// Ignore nodes that are marked unschedulable
	if node.Spec.Unschedulable {
		return false
	}
	return true
}

// GetReadinessState gets readiness state for the node
func GetReadinessState(node *apiv1.Node) (isNodeReady bool, lastTransitionTime time.Time, err error) {
	canNodeBeReady, readyFound := true, false
	lastTransitionTime = time.Time{}

	for _, cond := range node.Status.Conditions {
		switch cond.Type {
		case apiv1.NodeReady:
			readyFound = true
			if cond.Status == apiv1.ConditionFalse || cond.Status == apiv1.ConditionUnknown {
				canNodeBeReady = false
			}
			if lastTransitionTime.Before(cond.LastTransitionTime.Time) {
				lastTransitionTime = cond.LastTransitionTime.Time
			}
		case apiv1.NodeDiskPressure:
			if cond.Status == apiv1.ConditionTrue {
				canNodeBeReady = false
			}
			if lastTransitionTime.Before(cond.LastTransitionTime.Time) {
				lastTransitionTime = cond.LastTransitionTime.Time
			}
		case apiv1.NodeNetworkUnavailable:
			if cond.Status == apiv1.ConditionTrue {
				canNodeBeReady = false
			}
			if lastTransitionTime.Before(cond.LastTransitionTime.Time) {
				lastTransitionTime = cond.LastTransitionTime.Time
			}
		}
	}
	if !readyFound {
		return false, time.Time{}, fmt.Errorf("readiness information not found")
	}
	if !IsNodeExtendedReady(node) {
		canNodeBeReady = false
	}
	return canNodeBeReady, lastTransitionTime, nil
}

// GetUnreadyNodeCopy create a copy of the given node and override its NodeReady condition to False
func GetUnreadyNodeCopy(node *apiv1.Node) *apiv1.Node {
	newNode := node.DeepCopy()
	newReadyCondition := apiv1.NodeCondition{
		Type:               apiv1.NodeReady,
		Status:             apiv1.ConditionFalse,
		LastTransitionTime: node.CreationTimestamp,
	}
	newNodeConditions := []apiv1.NodeCondition{newReadyCondition}
	for _, condition := range newNode.Status.Conditions {
		if condition.Type != apiv1.NodeReady {
			newNodeConditions = append(newNodeConditions, condition)
		}
	}
	newNode.Status.Conditions = newNodeConditions
	return newNode
}

// IsNodeExtendedReady returns true if the node was marked ready by the node readiness controller
func IsNodeExtendedReady(node *apiv1.Node) bool {
	for _, taint := range node.Spec.Taints {
		if taint.Key == nodeNotReadyZalandoTaint {
			return false
		}
	}
	return true
}
