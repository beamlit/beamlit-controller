package config

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/beamlit/operator/api/v1alpha1"
	"gopkg.in/yaml.v2"
)

type RemoteDeploymentPlatformType string

const (
	RemoteDeploymentPlatformTypeKubernetes RemoteDeploymentPlatformType = "kubernetes"
	RemoteDeploymentPlatformTypeBeamLit    RemoteDeploymentPlatformType = "beamlit"
)

// Config is the configuration for the operator.
type Config struct {
	// MetricsAddr is the address the metric endpoint binds to.
	MetricsAddr *string `json:"metrics_addr,omitempty" yaml:"metricsAddr,omitempty"`
	// EnableLeaderElection enables leader election for controller manager.
	EnableLeaderElection *bool `json:"enable_leader_election,omitempty" yaml:"enableLeaderElection,omitempty"`
	// ProbeAddr is the address the probe endpoint binds to.
	ProbeAddr *string `json:"probe_addr,omitempty" yaml:"probeAddr,omitempty"`
	// SecureMetrics indicates if the metrics endpoint is served securely.
	SecureMetrics *bool `json:"secure_metrics,omitempty" yaml:"secureMetrics,omitempty"`
	// EnableHTTP2 indicates if HTTP/2 should be enabled for the metrics and webhook servers.
	EnableHTTP2 *bool `json:"enable_http2,omitempty" yaml:"enableHTTP2,omitempty"`
	// Namespaces is the list of namespaces to watch.
	Namespaces *string `json:"namespaces,omitempty" yaml:"namespaces,omitempty"`
	// Proxy is the configuration for the proxy service.
	ProxyService ProxyServiceConfig `json:"proxy_service,omitempty" yaml:"proxyService,omitempty"`
	// DefaultRemoteBackend is the configuration for the default remote backend service.
	DefaultRemoteBackend struct {
		// Host is the host of the remote backend
		Host *string `json:"host,omitempty" yaml:"host,omitempty"`

		// AuthConfig is the authentication configuration for the remote backend
		AuthConfig *v1alpha1.AuthConfig `json:"auth_config,omitempty" yaml:"authConfig,omitempty"`

		// PathPrefix is the path prefix for the remote backend
		PathPrefix *string `json:"path_prefix,omitempty" yaml:"pathPrefix,omitempty"`

		// HeadersToAdd is the list of headers to add to the requests
		HeadersToAdd map[string]string `json:"headers,omitempty" yaml:"headers,omitempty"`

		Scheme *v1alpha1.SupportedScheme `json:"scheme,omitempty" yaml:"scheme,omitempty"`
	} `json:"default_remote_backend,omitempty" yaml:"defaultRemoteBackend,omitempty"`

	Deployment struct {
		Enabled       *bool `json:"enabled,omitempty" yaml:"enabled,omitempty"` // TODO: use this to disable beamlit deployment
		Configuration struct {
			Type    *RemoteDeploymentPlatformType `json:"type,omitempty" yaml:"type,omitempty"`
			BeamLit struct{}                      `json:"beamlit,omitempty" yaml:"beamlit,omitempty"`
		} `json:"configuration,omitempty" yaml:"configuration,omitempty"`
	} `json:"deployment,omitempty" yaml:"deployment,omitempty"`
}

type ProxyServiceConfig struct {
	// Namespace is the namespace of the proxy service.
	Namespace *string `json:"namespace,omitempty" yaml:"namespace,omitempty"`
	// Name is the name of the proxy service.
	Name *string `json:"name,omitempty" yaml:"name,omitempty"`
	// Port is the port of the proxy service.
	Port *int `json:"port,omitempty" yaml:"port,omitempty"`
	// AdminPort is the admin port of the proxy service.
	AdminPort *int `json:"admin_port,omitempty" yaml:"adminPort,omitempty"`
}

func (c *Config) Validate() error {
	if c.ProxyService.Namespace == nil || c.ProxyService.Name == nil || c.ProxyService.Port == nil || c.ProxyService.AdminPort == nil {
		return fmt.Errorf("proxy service is not configured")
	}
	return nil
}

func (c *Config) Default() {
	c.EnableHTTP2 = toPointer(false)
	c.SecureMetrics = toPointer(false)
	c.Namespaces = toPointer("default")
	c.EnableLeaderElection = toPointer(false)
	c.MetricsAddr = toPointer(":8080")
	c.ProbeAddr = toPointer(":8081")
}

func (c *Config) FromFile(name string, reader io.Reader) error {
	if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
		return yaml.NewDecoder(reader).Decode(c)
	}
	return json.NewDecoder(reader).Decode(c)
}
