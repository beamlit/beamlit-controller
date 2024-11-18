package v1alpha1

type Route struct {
	Name      string    `json:"name" yaml:"name"`
	Hostnames []string  `json:"hostnames" yaml:"hostnames"`
	Backends  []Backend `json:"backends" yaml:"backends"`
}
