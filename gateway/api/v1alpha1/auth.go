package v1alpha1

type AuthType string

const (
	AuthTypeOAuth AuthType = "oauth"
)

type Auth struct {
	Type  AuthType `json:"type" yaml:"type"`
	OAuth *OAuth   `json:"oauth" yaml:"oauth"`
}

type OAuth struct {
	ClientID     string `json:"clientId" yaml:"clientId"`
	ClientSecret string `json:"clientSecret" yaml:"clientSecret"`
	TokenURL     string `json:"token_url" yaml:"token_url"`
}
