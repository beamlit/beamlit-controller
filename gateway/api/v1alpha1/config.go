package v1alpha1

// Config is the configuration for the Beamlit Proxy API
type Config struct {
	BackendAddr string `json:"backend_addr" yaml:"backend_addr"`
	RemoteAddr  string `json:"remote_addr" yaml:"remote_addr"`
	// Percent is the percentage of requests to proxy to the remote address
	Percent int `json:"percent" yaml:"percent"`
}
