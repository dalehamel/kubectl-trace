/*


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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// TODO: Make the Spec immutable (guard it with a admission controller)
// TODO: add defaulting / validations
// TOOD: for defaulting? GA in 1.17 for basic functionality from controller-tools. before that it requires admission controller.

// TraceJobSpec defines the desired state of TraceJob
type TraceJobSpec struct {
	TargetNamespace string `json:"targetNamespace"`

	Resource  string `json:"resource"`            // conceptually arg[0]
	Container string `json:"container,omitempty"` // conceptually arg[1] / "-c"

	// +kubebuilder:validation:Enum=bpftrace;bcc;fake;rbspy
	Tracer   string `json:"tracer,omitempty"`
	Selector string `json:"selector,omitempty"`

	// +kubebuilder:default=stdout
	// +kubebuilder:validation:Pattern=`^(gs:\/\/.+)|(stdout)$`
	Output string `json:"output,omitempty"`

	Program          string   `json:"program,omitempty"`
	ProgramArgs      []string `json:"programArgs,omitempty"`
	ImageNameTag     string   `json:"imageNameTag,omitempty"`
	InitImageNameTag string   `json:"initImageNameTag,omitempty"`
	FetchHeaders     bool     `json:"fetchHeaders,omitempty"`
	GoogleAppSecret  string   `json:"googleAppSecret,omitempty"`

	// +kubebuilder:default=3600
	Deadline int64 `json:"deadline,omitempty"`

	// +kubebuilder:default=30
	DeadlineGracePeriod int64 `json:"deadlineGracePeriod,omitempty"`
}

// TraceJobStatus defines the observed state of TraceJob
type TraceJobStatus struct {
	ArtifactPath string            `json:"artifactPath,omitempty"`
	JobName      string            `json:"jobName,omitempty"`
	Conditions   []StatusCondition `json:"conditions,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status

// TraceJob is the Schema for the tracejobs API
type TraceJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   TraceJobSpec   `json:"spec,omitempty"`
	Status TraceJobStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// TraceJobList contains a list of TraceJob
type TraceJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []TraceJob `json:"items"`
}

func init() {
	SchemeBuilder.Register(&TraceJob{}, &TraceJobList{})
}
