package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ConditionType string
type ConditionStatus string

var (
	ConditionStatusTrue    ConditionStatus = "True"
	ConditionStatusFalse   ConditionStatus = "False"
	ConditionStatusUnknown ConditionStatus = "Unknown"
)

var (
	ConditionTypeRunning ConditionType = "Running"
	ConditionTypeSuccess ConditionType = "Succeeded"
	ConditionTypeFailed  ConditionType = "Failed"
	ConditionTypeInvalid ConditionType = "Invalid"
)

type StatusCondition struct {
	Type ConditionType `json:"type"`

	// Status of the condition, one of True, False, Unknown.
	Status ConditionStatus `json:"status"`
	// +optional
	LastProbeTime metav1.Time `json:"lastProbeTime,omitempty"`
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
	// +optional
	Reason string `json:"reason,omitempty"`
	// +optional
	Message string `json:"message,omitempty"`
}

func SetCondition(conds *[]StatusCondition, targetCond *StatusCondition) {
	var outCond *StatusCondition

	for i, cond := range *conds {
		if cond.Type == targetCond.Type {
			outCond = &(*conds)[i]
			break
		}
	}

	if outCond == nil {
		*conds = append(*conds, *targetCond)
		outCond = &(*conds)[len(*conds)-1]
		outCond.LastTransitionTime = metav1.Now()
	} else {
		lastStatus := outCond.Status
		lastTrans := outCond.LastTransitionTime
		*outCond = *targetCond
		if outCond.Status != lastStatus {
			outCond.LastTransitionTime = metav1.Now()
		} else {
			outCond.LastTransitionTime = lastTrans
		}
	}

	outCond.LastProbeTime = metav1.Now()
}
