/*
Copyright 2024.

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

package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type AWSVPCSpec struct {
	Region     string `json:"region"`
	CIDRBlock  string `json:"cidrBlock"`
	Name       string `json:"name"`
	SubnetCIDR string `json:"subnetCIDR"`
}

type AWSVPCStatus struct {
	VPCID        string `json:"vpcId,omitempty"`
	SubnetID     string `json:"subnetId,omitempty"`
	Status       string `json:"status,omitempty"`
	ErrorMessage string `json:"errorMessage,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

type AWSVPC struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AWSVPCSpec   `json:"spec,omitempty"`
	Status AWSVPCStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

type AWSVPCList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []AWSVPC `json:"items"`
}

func init() {
	SchemeBuilder.Register(&AWSVPC{}, &AWSVPCList{})
}
