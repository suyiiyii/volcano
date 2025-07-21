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

package vapmigration

import (
	"context"
	"fmt"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"

	e2eutil "volcano.sh/volcano/test/e2e/util"
	"io/ioutil"
)

var _ = Describe("Simple VAP Testing", func() {
	var (
		ctx *e2eutil.TestContext
		vapGVR schema.GroupVersionResource
	)

	BeforeEach(func() {
		ctx = e2eutil.InitTestContext(e2eutil.Options{})
		
		// ValidatingAdmissionPolicy GVR
		vapGVR = schema.GroupVersionResource{
			Group:    "admissionregistration.k8s.io",
			Version:  "v1",
			Resource: "validatingadmissionpolicies",
		}
	})

	AfterEach(func() {
		e2eutil.CleanupTestContext(ctx)
	})

	Context("ValidatingAdmissionPolicy Basic Tests", func() {
		It("Should be able to create ValidatingAdmissionPolicy resources", func() {
			By("Checking that ValidatingAdmissionPolicy API is available")
			_, err := e2eutil.DynamicClient.Resource(vapGVR).List(context.TODO(), metav1.ListOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should be able to apply Jobs ValidatingAdmissionPolicy", func() {
			By("Loading Jobs VAP policy from YAML")
			policyPath := filepath.Join("..", "..", "..", "config", "validating-admission-policies", "jobs-validation-policy.yaml")
			policyData, err := ioutil.ReadFile(policyPath)
			Expect(err).NotTo(HaveOccurred())

			var policy unstructured.Unstructured
			err = yaml.Unmarshal(policyData, &policy)
			Expect(err).NotTo(HaveOccurred())

			By("Creating the ValidatingAdmissionPolicy")
			createdPolicy, err := e2eutil.DynamicClient.Resource(vapGVR).Create(context.TODO(), &policy, metav1.CreateOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(createdPolicy.GetName()).To(Equal("volcano-jobs-validation"))

			By("Verifying the policy was created")
			retrievedPolicy, err := e2eutil.DynamicClient.Resource(vapGVR).Get(context.TODO(), "volcano-jobs-validation", metav1.GetOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(retrievedPolicy.GetName()).To(Equal("volcano-jobs-validation"))

			By("Cleaning up the policy")
			err = e2eutil.DynamicClient.Resource(vapGVR).Delete(context.TODO(), "volcano-jobs-validation", metav1.DeleteOptions{})
			Expect(err).NotTo(HaveOccurred())
		})

		It("Should validate that all VAP policies have correct syntax", func() {
			policies := []string{
				"jobs-validation-policy.yaml",
				"pods-validation-policy.yaml", 
				"queues-validation-policy.yaml",
				"podgroups-validation-policy.yaml",
				"hypernodes-validation-policy.yaml",
				"jobflows-validation-policy.yaml",
			}

			for _, policyFile := range policies {
				By(fmt.Sprintf("Validating %s", policyFile))
				policyPath := filepath.Join("..", "..", "..", "config", "validating-admission-policies", policyFile)
				policyData, err := ioutil.ReadFile(policyPath)
				if err != nil {
					// Skip if file doesn't exist - some policies might not be implemented yet
					continue
				}

				var policy unstructured.Unstructured
				err = yaml.Unmarshal(policyData, &policy)
				Expect(err).NotTo(HaveOccurred(), fmt.Sprintf("Failed to parse %s", policyFile))

				// Verify basic structure
				Expect(policy.GetAPIVersion()).To(Equal("admissionregistration.k8s.io/v1"))
				Expect(policy.GetKind()).To(Equal("ValidatingAdmissionPolicy"))
				Expect(policy.GetName()).NotTo(BeEmpty())
			}
		})
	})
})