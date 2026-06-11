/*
Copyright 2026.

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

package controller

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAllSubConditionsReady(t *testing.T) {
	tests := []struct {
		name       string
		conditions []metav1.Condition
		want       bool
	}{
		{
			name:       "empty conditions list is vacuously true",
			conditions: []metav1.Condition{},
			want:       true,
		},
		{
			name: "all sub-conditions true with no Ready condition",
			conditions: []metav1.Condition{
				{Type: ConditionDatabaseReady, Status: metav1.ConditionTrue},
				{Type: ConditionStatefulSetReady, Status: metav1.ConditionTrue},
				{Type: ConditionServiceReady, Status: metav1.ConditionTrue},
			},
			want: true,
		},
		{
			name: "all sub-conditions true with Ready=False is still true (the fix)",
			conditions: []metav1.Condition{
				{Type: ConditionDatabaseReady, Status: metav1.ConditionTrue},
				{Type: ConditionStatefulSetReady, Status: metav1.ConditionTrue},
				{Type: ConditionServiceReady, Status: metav1.ConditionTrue},
				{Type: ConditionReady, Status: metav1.ConditionFalse},
			},
			want: true,
		},
		{
			name: "one sub-condition false returns false",
			conditions: []metav1.Condition{
				{Type: ConditionDatabaseReady, Status: metav1.ConditionTrue},
				{Type: ConditionStatefulSetReady, Status: metav1.ConditionFalse},
				{Type: ConditionServiceReady, Status: metav1.ConditionTrue},
			},
			want: false,
		},
		{
			name: "only Ready condition present is vacuously true",
			conditions: []metav1.Condition{
				{Type: ConditionReady, Status: metav1.ConditionFalse},
			},
			want: true,
		},
		{
			name: "advisory conditions do not gate readiness",
			conditions: []metav1.Condition{
				{Type: ConditionDatabaseReady, Status: metav1.ConditionTrue},
				{Type: ConditionStatefulSetReady, Status: metav1.ConditionTrue},
				{Type: ConditionWorkloadProfileValid, Status: metav1.ConditionFalse},
				{Type: ConditionMultiReplicaPreconditions, Status: metav1.ConditionFalse},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := allSubConditionsReady(tt.conditions)
			if got != tt.want {
				t.Errorf("allSubConditionsReady() = %v, want %v", got, tt.want)
			}
		})
	}
}
