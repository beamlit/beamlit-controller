package beamlit

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

type BeamlitToken struct {
	clientID     string
	clientSecret string
	baseURL      string
	cfg          *clientcredentials.Config
}

// NewBeamlitToken creates a new BeamlitToken from the environment variables.
// ENV variables:
// BEAMLIT_TOKEN: base64 encoded string of clientID:clientSecret
// BEAMLIT_BASE_URL: base URL of the Beamlit API
func NewBeamlitToken() (*BeamlitToken, error) {
	clientID, clientSecret, baseURL, err := retrieveInfoFromEnv()
	if err != nil {
		return nil, err
	}

	return &BeamlitToken{
		clientID:     clientID,
		clientSecret: clientSecret,
		baseURL:      baseURL,
	}, nil
}

// retrieveInfoFromEnv retrieves the clientID, clientSecret, and baseURL from the environment variables.
// It returns the clientID, clientSecret, and baseURL as strings.
// It returns no error if the environment variables are not set.
// It returns an error if the Beamlit token is not base64 encoded, or if the token is not in the format clientID:clientSecret
func retrieveInfoFromEnv() (string, string, string, error) {
	var baseURL string
	var beamlitToken string

	if baseURL = os.Getenv(envBaseURL); baseURL == "" {
		baseURL = defaultBaseURL
	}

	if beamlitToken = os.Getenv(envToken); beamlitToken == "" {
		return "", "", "", nil
	}

	decodedToken, err := base64.StdEncoding.DecodeString(beamlitToken)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to decode Beamlit token: %w", err)
	}

	splitToken := strings.Split(string(decodedToken), ":")
	if len(splitToken) != 2 {
		return "", "", "", fmt.Errorf("invalid Beamlit token format")
	}

	return splitToken[0], splitToken[1], baseURL, nil
}

// client is set private to prevent users from using it directly.
// Use NewClient() to get a new client.
func (b *BeamlitToken) client(ctx context.Context) *http.Client {
	if b.cfg == nil {
		b.cfg = &clientcredentials.Config{
			ClientID:     b.clientID,
			ClientSecret: b.clientSecret,
			TokenURL:     fmt.Sprintf("%s/oauth/token", b.baseURL),
			AuthStyle:    oauth2.AuthStyleInHeader,
		}
	}

	return b.cfg.Client(ctx)
}

// GetToken retrieves the access token, and refreshes it if it is expired, from the Beamlit API using the client credentials flow
// It returns the access token as a string. The token is not refreshed automatically,
// so the caller must refresh the token when it is expired by calling this function again.
func (b *BeamlitToken) GetToken(ctx context.Context) (string, error) {
	if b.cfg == nil {
		b.cfg = &clientcredentials.Config{
			ClientID:     b.clientID,
			ClientSecret: b.clientSecret,
			TokenURL:     fmt.Sprintf("%s/oauth/token", b.baseURL),
			AuthStyle:    oauth2.AuthStyleInHeader,
		}
	}
	token, err := b.cfg.Token(ctx)
	if err != nil {
		return "", err
	}
	return token.AccessToken, nil
}
