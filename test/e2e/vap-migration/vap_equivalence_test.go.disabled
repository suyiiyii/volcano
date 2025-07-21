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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"volcano.sh/volcano/test/e2e/vap-migration/util"
	e2eutil "volcano.sh/volcano/test/e2e/util"
)

var _ = Describe("VAP-Webhook Equivalence Testing", func() {
	var (
		ctx            *e2eutil.TestContext
		webhookClient  *util.WebhookTestClient
		vapClient      *util.VAPTestClient
		comparisonEngine *util.ComparisonEngine
		metricsCollector *util.MetricsCollector
	)

	BeforeEach(func() {
		ctx = e2eutil.InitTestContext(e2eutil.Options{})
		
		webhookClient = util.NewWebhookTestClient(e2eutil.KubeClient, e2eutil.VcClient)
		vapClient = util.NewVAPTestClient(e2eutil.KubeClient, e2eutil.DynamicClient)
		comparisonEngine = util.NewComparisonEngine(true) // strict mode
		metricsCollector = util.NewMetricsCollector()
	})

	AfterEach(func() {
		// Generate test report
		report := metricsCollector.GenerateReport()
		fmt.Printf("\n=== VAP-Webhook Equivalence Report ===\n%s\n", report)
		
		e2eutil.CleanupTestContext(ctx)
	})

	Context("Jobs Validation Equivalence", func() {
		It("Should validate basic job fields identically", func() {
			testScenarios := util.LoadJobTestScenarios()
			
			for _, scenario := range testScenarios.BasicFieldValidation {
				By(fmt.Sprintf("Testing scenario: %s", scenario.Name))
				
				// Test webhook validation
				webhookResult, err := webhookClient.ValidateJob(ctx.Namespace, scenario.JobSpec)
				Expect(err).NotTo(HaveOccurred())
				
				// Test VAP validation
				vapResult, err := vapClient.ValidateJob(ctx.Namespace, scenario.JobSpec)
				Expect(err).NotTo(HaveOccurred())
				
				// Compare results
				comparison := comparisonEngine.CompareJobValidation(webhookResult, vapResult)
				metricsCollector.RecordJobComparison(scenario.Name, comparison)
				
				Expect(comparison.Match).To(BeTrue(), 
					fmt.Sprintf("Scenario %s failed: %v", scenario.Name, comparison.Differences))
				
				// Verify expected result matches both
				if scenario.Expected.Allowed {
					Expect(webhookResult.Allowed).To(BeTrue(), "Webhook should allow valid job")
					Expect(vapResult.Allowed).To(BeTrue(), "VAP should allow valid job")
				} else {
					Expect(webhookResult.Allowed).To(BeFalse(), "Webhook should reject invalid job")
					Expect(vapResult.Allowed).To(BeFalse(), "VAP should reject invalid job")
					
					// Verify error messages are equivalent
					Expect(webhookResult.ErrorMessage).To(ContainSubstring(scenario.Expected.ErrorContains))
					Expect(vapResult.ErrorMessage).To(ContainSubstring(scenario.Expected.ErrorContains))
				}
			}
		})

		It("Should handle cross-field validation identically", func() {
			testScenarios := util.LoadJobTestScenarios()
			
			for _, scenario := range testScenarios.CrossFieldValidation {
				By(fmt.Sprintf("Testing cross-field scenario: %s", scenario.Name))
				
				webhookResult, _ := webhookClient.ValidateJob(ctx.Namespace, scenario.JobSpec)
				vapResult, _ := vapClient.ValidateJob(ctx.Namespace, scenario.JobSpec)
				
				comparison := comparisonEngine.CompareJobValidation(webhookResult, vapResult)
				metricsCollector.RecordJobComparison(scenario.Name, comparison)
				
				Expect(comparison.Match).To(BeTrue())
				Expect(webhookResult.Allowed).To(Equal(vapResult.Allowed))
			}
		})

		It("Should validate task structure identically", func() {
			testScenarios := util.LoadJobTestScenarios()
			
			for _, scenario := range testScenarios.TaskStructureValidation {
				By(fmt.Sprintf("Testing task structure scenario: %s", scenario.Name))
				
				webhookResult, _ := webhookClient.ValidateJob(ctx.Namespace, scenario.JobSpec)
				vapResult, _ := vapClient.ValidateJob(ctx.Namespace, scenario.JobSpec)
				
				comparison := comparisonEngine.CompareJobValidation(webhookResult, vapResult)
				Expect(comparison.Match).To(BeTrue())
			}
		})
	})

	Context("Performance Comparison", func() {
		It("Should maintain similar latency characteristics", func() {
			performanceTest := util.NewPerformanceTest(webhookClient, vapClient)
			results := performanceTest.RunJobValidationComparison(100)
			
			By("Comparing P95 latencies")
			webhookP95 := results.WebhookLatency.P95()
			vapP95 := results.VAPLatency.P95()
			tolerance := time.Duration(float64(webhookP95) * 0.1) // 10% tolerance
			
			Expect(vapP95).To(BeNumerically("~", webhookP95, tolerance),
				fmt.Sprintf("VAP P95 latency (%v) should be within 10%% of webhook P95 (%v)", vapP95, webhookP95))
			
			By("Recording performance metrics")
			metricsCollector.RecordPerformanceComparison("jobs", results)
		})

		It("Should handle burst load equivalently", func() {
			performanceTest := util.NewPerformanceTest(webhookClient, vapClient)
			burstResults := performanceTest.RunBurstLoadTest(50, 30*time.Second)
			
			// Verify both webhook and VAP handle the load similarly
			Expect(burstResults.WebhookErrorRate).To(BeNumerically("~", burstResults.VAPErrorRate, 0.05))
			
			metricsCollector.RecordBurstLoadComparison(burstResults)
		})
	})

	Context("Edge Case Testing", func() {
		It("Should handle complex job configurations identically", func() {
			// Test maximum task counts, complex plugin configs, etc.
			edgeCases := util.LoadJobEdgeCases()
			
			for _, edgeCase := range edgeCases {
				webhookResult, _ := webhookClient.ValidateJob(ctx.Namespace, edgeCase.JobSpec)
				vapResult, _ := vapClient.ValidateJob(ctx.Namespace, edgeCase.JobSpec)
				
				comparison := comparisonEngine.CompareJobValidation(webhookResult, vapResult)
				Expect(comparison.Match).To(BeTrue(),
					fmt.Sprintf("Edge case %s: %v", edgeCase.Name, comparison.Differences))
			}
		})
	})
})