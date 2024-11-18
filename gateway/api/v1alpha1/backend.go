package v1alpha1

type Backend struct {
	// Host is the host of the backend service.
	// Can include the port, e.g. example.com:8080
	Host         string            `json:"host" yaml:"host"`
	Weight       int               `json:"weight" yaml:"weight"`
	Auth         *Auth             `json:"auth" yaml:"auth"`
	PathPrefix   string            `json:"path_prefix" yaml:"path_prefix"`
	HeadersToAdd map[string]string `json:"headers" yaml:"headers"`
	Scheme       string            `json:"scheme" yaml:"scheme"` // http or https
}
