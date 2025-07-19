/*
Copyright 2024 The Volcano Authors.

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

package util

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"

	vcclient "volcano.sh/apis/pkg/client/clientset/versioned"
	"volcano.sh/apis/pkg/apis/batch/v1alpha1"
)

// ValidationResult contains the result of a validation operation
type ValidationResult struct {
	Allowed      bool
	ErrorMessage string
	Warnings     []string
	Latency      time.Duration
	Annotations  map[string]string
}

// WebhookTestClient manages webhook-specific testing
type WebhookTestClient struct {
	kubeClient    kubernetes.Interface
	volcanoClient vcclient.Interface
}

// NewWebhookTestClient creates a new webhook test client
func NewWebhookTestClient(kubeClient kubernetes.Interface, volcanoClient vcclient.Interface) *WebhookTestClient {
	return &WebhookTestClient{
		kubeClient:    kubeClient,
		volcanoClient: volcanoClient,
	}
}

// ValidateJob validates a job using webhook validation (with webhooks enabled)
func (w *WebhookTestClient) ValidateJob(namespace string, jobSpec *v1alpha1.JobSpec) (*ValidationResult, error) {
	startTime := time.Now()
	
	// Create job object for validation
	job := &v1alpha1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch.volcano.sh/v1alpha1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-job-%d", time.Now().UnixNano()),
			Namespace: namespace,
		},
		Spec: *jobSpec,
	}

	// Convert to unstructured for dry-run creation
	jobBytes, err := json.Marshal(job)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal job: %v", err)
	}

	var jobUnstructured unstructured.Unstructured
	if err := json.Unmarshal(jobBytes, &jobUnstructured); err != nil {
		return nil, fmt.Errorf("failed to unmarshal to unstructured: %v", err)
	}

	// Create with dry-run=server to trigger webhook validation
	_, err = w.volcanoClient.BatchV1alpha1().Jobs(namespace).Create(
		context.Background(), 
		job, 
		metav1.CreateOptions{DryRun: []string{metav1.DryRunAll}},
	)

	result := &ValidationResult{
		Latency: time.Since(startTime),
	}

	if err != nil {
		result.Allowed = false
		result.ErrorMessage = err.Error()
	} else {
		result.Allowed = true
	}

	return result, nil
}

// ValidatePod validates a pod using webhook validation
func (w *WebhookTestClient) ValidatePod(namespace string, podSpec interface{}) (*ValidationResult, error) {
	startTime := time.Now()
	
	// Implementation for pod validation
	// Similar structure to ValidateJob but for pods
	
	result := &ValidationResult{
		Latency: time.Since(startTime),
		Allowed: true, // Placeholder
	}
	
	return result, nil
}

// VAPTestClient manages VAP-specific testing
type VAPTestClient struct {
	kubeClient kubernetes.Interface
}

// NewVAPTestClient creates a new VAP test client
func NewVAPTestClient(kubeClient kubernetes.Interface) *VAPTestClient {
	return &VAPTestClient{
		kubeClient: kubeClient,
	}
}

// ValidateJob validates a job using VAP validation (with VAP enabled, webhooks disabled)
func (v *VAPTestClient) ValidateJob(namespace string, jobSpec *v1alpha1.JobSpec) (*ValidationResult, error) {
	startTime := time.Now()
	
	// Temporarily disable webhook for this test
	// This would require webhook configuration management
	
	// Create job object for validation
	job := &v1alpha1.Job{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "batch.volcano.sh/v1alpha1",
			Kind:       "Job",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("test-vap-job-%d", time.Now().UnixNano()),
			Namespace: namespace,
		},
		Spec: *jobSpec,
	}

	// Convert to runtime.Object and then to unstructured
	jobObj := job.DeepCopyObject()
	jobUnstructured, err := runtime.DefaultUnstructuredConverter.ToUnstructured(jobObj)
	if err != nil {
		return nil, fmt.Errorf("failed to convert job to unstructured: %v", err)
	}

	unstruct := &unstructured.Unstructured{Object: jobUnstructured}
	
	// Create with dry-run=server to trigger VAP validation
	gvr := v1alpha1.SchemeGroupVersion.WithResource("jobs")
	_, err = v.kubeClient.RESTClient().Post().
		AbsPath("/api", gvr.Group, gvr.Version, "namespaces", namespace, gvr.Resource).
		Param("dryRun", metav1.DryRunAll).
		Body(unstruct).
		Do(context.Background()).Get()

	result := &ValidationResult{
		Latency: time.Since(startTime),
	}

	if err != nil {
		result.Allowed = false
		result.ErrorMessage = err.Error()
	} else {
		result.Allowed = true
	}

	return result, nil
}

// ValidatePod validates a pod using VAP validation
func (v *VAPTestClient) ValidatePod(namespace string, podSpec interface{}) (*ValidationResult, error) {
	startTime := time.Now()
	
	// Implementation for pod validation using VAP
	// Similar structure to ValidateJob but for pods
	
	result := &ValidationResult{
		Latency: time.Since(startTime),
		Allowed: true, // Placeholder
	}
	
	return result, nil
}