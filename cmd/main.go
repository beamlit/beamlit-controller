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

package main

import (
	"crypto/tls"
	"flag"
	"os"
	"strings"
	"sync"
	"time"

	// Import all Kubernetes client auth plugins (e.g. Azure, GCP, OIDC, etc.)
	// to ensure that exec-entrypoint and run can make use of them.

	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/cache"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	modelv1alpha1 "github.com/beamlit/operator/api/v1alpha1"
	"github.com/beamlit/operator/internal/beamlit"
	"github.com/beamlit/operator/internal/controller"
	"github.com/beamlit/operator/internal/dataplane/configurer"
	"github.com/beamlit/operator/internal/dataplane/offloader"
	"github.com/beamlit/operator/internal/informers/health"
	"github.com/beamlit/operator/internal/informers/metric"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	gatewayclient "sigs.k8s.io/gateway-api/pkg/client/clientset/versioned"
	//+kubebuilder:scaffold:imports
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(modelv1alpha1.AddToScheme(scheme))
	//+kubebuilder:scaffold:scheme
}

func main() {
	var metricsAddr string
	var enableLeaderElection bool
	var probeAddr string
	var secureMetrics bool
	var enableHTTP2 bool
	var namespaces string
	var scrapeInterval time.Duration

	var beamlitGatewayAddress string
	var proxyListenPort int

	var defaultRemoteServiceRefNamespace string
	var defaultRemoteServiceRefName string
	var defaultRemoteServiceRefTargetPort int

	var gatewayServiceRefNamespace string
	var gatewayServiceRefName string
	var gatewayServiceRefTargetPort int

	var gatewayName string
	var gatewayNamespace string

	flag.StringVar(&metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.BoolVar(&secureMetrics, "metrics-secure", false,
		"If set the metrics endpoint is served securely")
	flag.BoolVar(&enableHTTP2, "enable-http2", false,
		"If set, HTTP/2 will be enabled for the metrics and webhook servers")
	flag.StringVar(&namespaces, "namespaces", "", "The namespaces to watch for resources (comma separated)")

	flag.DurationVar(&scrapeInterval, "scrape-interval", 30*time.Second, "The interval at which to scrape metrics")
	flag.StringVar(&beamlitGatewayAddress, "beamlit-gateway-address", "0.0.0.0", "The address the beamlit gateway binds to.")
	flag.IntVar(&proxyListenPort, "proxy-listen-port", 8000, "The port the proxy listens on.")
	flag.StringVar(&defaultRemoteServiceRefNamespace, "default-remote-service-ref-namespace", "default", "The namespace of the default remote service reference.")
	flag.StringVar(&defaultRemoteServiceRefName, "default-remote-service-ref-name", "default", "The name of the default remote service reference.")
	flag.IntVar(&defaultRemoteServiceRefTargetPort, "default-remote-service-ref-target-port", 8000, "The target port of the default remote service reference.")
	flag.StringVar(&gatewayServiceRefNamespace, "gateway-service-ref-namespace", "default", "The namespace of the gateway service reference.")
	flag.StringVar(&gatewayServiceRefName, "gateway-service-ref-name", "default", "The name of the gateway service reference.")
	flag.IntVar(&gatewayServiceRefTargetPort, "gateway-service-ref-target-port", 8000, "The target port of the gateway service reference.")
	flag.StringVar(&gatewayName, "gateway-name", "beamlit-gateway", "The name of the gateway.")
	flag.StringVar(&gatewayNamespace, "gateway-namespace", "beamlit", "The namespace of the gateway.")
	opts := zap.Options{
		Development: true,
	}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	// if the enable-http2 flag is false (the default), http/2 should be disabled
	// due to its vulnerabilities. More specifically, disabling http/2 will
	// prevent from being vulnerable to the HTTP/2 Stream Cancellation and
	// Rapid Reset CVEs. For more information see:
	// - https://github.com/advisories/GHSA-qppj-fm5r-hxr3
	// - https://github.com/advisories/GHSA-4374-p667-p6c8
	disableHTTP2 := func(c *tls.Config) {
		setupLog.Info("disabling http/2")
		c.NextProtos = []string{"http/1.1"}
	}

	tlsOpts := []func(*tls.Config){}
	if !enableHTTP2 {
		tlsOpts = append(tlsOpts, disableHTTP2)
	}

	webhookServer := webhook.NewServer(webhook.Options{
		TLSOpts: tlsOpts,
	})

	ctrlOpts := ctrl.Options{
		Scheme: scheme,
		Metrics: metricsserver.Options{
			BindAddress:   metricsAddr,
			SecureServing: secureMetrics,
			TLSOpts:       tlsOpts,
		},
		WebhookServer:          webhookServer,
		HealthProbeBindAddress: probeAddr,
		LeaderElection:         enableLeaderElection,
		LeaderElectionID:       "0e22b10b.beamlit.io",
	}

	namespacesList := make(map[string]cache.Config)
	if namespaces != "" {
		for _, ns := range strings.Split(namespaces, ",") {
			namespacesList[ns] = cache.Config{}
		}
	}

	if len(namespacesList) > 0 {
		ctrlOpts.NewCache = cache.New
		ctrlOpts.Cache = cache.Options{
			DefaultNamespaces: namespacesList,
		}
	}

	ctx := ctrl.SetupSignalHandler()

	config, err := ctrl.GetConfig()
	if err != nil {
		setupLog.Error(err, "unable to get config")
		os.Exit(1)
	}
	mgr, err := ctrl.NewManager(config, ctrlOpts)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}
	metricInformer, err := metric.NewMetricInformer(ctx, config, metric.K8SMetricInformerType)
	if err != nil {
		setupLog.Error(err, "unable to create metrics watcher")
		os.Exit(1)
	}
	metricChan := metricInformer.Start(ctx)

	healthInformer, err := health.NewHealthInformer(ctx, config, health.K8SHealthInformerType)
	if err != nil {
		setupLog.Error(err, "unable to create health watcher")
		os.Exit(1)
	}
	healthChan := healthInformer.Start(ctx)

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create clientset")
		os.Exit(1)
	}

	beamlitClient, err := beamlit.NewClient()
	if err != nil {
		setupLog.Error(err, "unable to create beamlit client")
		os.Exit(1)
	}

	configurer, err := configurer.NewConfigurer(ctx, configurer.KubernetesConfigurerType, clientset)
	if err != nil {
		setupLog.Error(err, "unable to create configurer")
		os.Exit(1)
	}

	go configurer.Start(ctx, &modelv1alpha1.ServiceReference{
		ObjectReference: corev1.ObjectReference{
			Namespace: defaultRemoteServiceRefNamespace,
			Name:      defaultRemoteServiceRefName,
		},
		TargetPort: int32(defaultRemoteServiceRefTargetPort),
	}, &modelv1alpha1.ServiceReference{
		ObjectReference: corev1.ObjectReference{
			Namespace: gatewayServiceRefNamespace,
			Name:      gatewayServiceRefName,
		},
		TargetPort: int32(gatewayServiceRefTargetPort),
	})

	gatewayClient, err := gatewayclient.NewForConfig(config)
	if err != nil {
		setupLog.Error(err, "unable to create gateway client")
		os.Exit(1)
	}

	offloader, err := offloader.NewOffloader(ctx, offloader.GatewayAPIOffloaderType, clientset, gatewayClient, gatewayName, gatewayNamespace)
	if err != nil {
		setupLog.Error(err, "unable to create offloader")
		os.Exit(1)
	}

	ctrl := &controller.ModelDeploymentReconciler{
		Client:             mgr.GetClient(),
		Scheme:             mgr.GetScheme(),
		BeamlitClient:      beamlitClient,
		MetricInformer:     metricInformer,
		MetricStatusChan:   metricChan,
		Configurer:         configurer,
		HealthInformer:     healthInformer,
		HealthStatusChan:   healthChan,
		Offloader:          offloader,
		ManagedModels:      make(map[string]v1.ObjectReference),
		OngoingOffloadings: sync.Map{},
		DefaultRemoteServiceRef: &modelv1alpha1.ServiceReference{
			ObjectReference: corev1.ObjectReference{
				Namespace: defaultRemoteServiceRefNamespace,
				Name:      defaultRemoteServiceRefName,
			},
			TargetPort: int32(defaultRemoteServiceRefTargetPort),
		},
		BeamlitModels: make(map[string]string),
	}

	if err = ctrl.SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "ModelDeployment")
		os.Exit(1)
	}
	//+kubebuilder:scaffold:builder

	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up health check")
		os.Exit(1)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		setupLog.Error(err, "unable to set up ready check")
		os.Exit(1)
	}

	go ctrl.WatchForInformerUpdates(ctx)

	go setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}
