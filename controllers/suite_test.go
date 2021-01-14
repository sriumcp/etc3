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

package controllers

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/go-logr/logr"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	v2alpha1 "github.com/iter8-tools/etc3/api/v2alpha1"
	"github.com/iter8-tools/etc3/configuration"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var lg logr.Logger
var reconciler *ExperimentReconciler

type testHTTP struct {
	analysis *v2alpha1.Analysis
}

func (t *testHTTP) Post(url, contentType string, body []byte) ([]byte, int, error) {
	statuscode := 200
	b, err := json.Marshal(t.analysis)
	if err != nil {
		statuscode = 500
	}
	return b, statuscode, err
}

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func(done Done) {
	logf.SetLogger(zap.LoggerTo(GinkgoWriter, true))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths: []string{filepath.Join("..", "config", "crd", "bases")},
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = v2alpha1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	// k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	// Expect(err).ToNot(HaveOccurred())
	// Expect(k8sClient).ToNot(BeNil())

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	// restCfg, err := config.GetConfig()
	// Expect(err).ToNot(HaveOccurred())

	cfg := configuration.NewIter8Config().
		WithStrategy(string(v2alpha1.StrategyTypeCanary), map[string]string{"start": "start", "finish": "finish", "rollback": "finish", "failure": "finish"}).
		WithStrategy(string(v2alpha1.StrategyTypeAB), map[string]string{"start": "start", "finish": "finish", "rollback": "finish", "failure": "finish"}).
		WithStrategy(string(v2alpha1.StrategyTypePerformance), map[string]string{"start": "start"}).
		WithStrategy(string(v2alpha1.StrategyTypeBlueGreen), map[string]string{"start": "start", "finish": "finish", "rollback": "finish", "failure": "finish"}).
		WithRequestCount("request-count").
		WithEndpoint("http://iter8-analytics:8080").
		Build()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	lg := ctrl.Log.WithName("controllers").WithName("Experiment")

	testTransport := &testHTTP{
		analysis: &v2alpha1.Analysis{
			AggregatedMetrics: &v2alpha1.AggregatedMetricsAnalysis{
				AnalysisMetaData: v2alpha1.AnalysisMetaData{},
				Data:             map[string]v2alpha1.AggregatedMetricsData{},
			},
			WinnerAssessment: &v2alpha1.WinnerAssessmentAnalysis{
				AnalysisMetaData: v2alpha1.AnalysisMetaData{},
				Data:             v2alpha1.WinnerAssessmentData{},
			},
			VersionAssessments: &v2alpha1.VersionAssessmentAnalysis{
				AnalysisMetaData: v2alpha1.AnalysisMetaData{},
				Data:             map[string]v2alpha1.BooleanList{},
			},
			Weights: &v2alpha1.WeightsAnalysis{
				AnalysisMetaData: v2alpha1.AnalysisMetaData{},
				Data:             []v2alpha1.WeightData{},
			},
		},
	}

	reconciler := &ExperimentReconciler{
		Client:        k8sClient,
		Log:           lg,
		Scheme:        k8sManager.GetScheme(),
		RestConfig:    nil, // restCfg,
		EventRecorder: k8sManager.GetEventRecorderFor("iter8"),
		Iter8Config:   cfg,
		HTTP:          testTransport,
	}

	Expect(reconciler.SetupWithManager(k8sManager)).Should(Succeed())

	go func() {
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	close(done)
}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
