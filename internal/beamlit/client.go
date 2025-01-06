package beamlit

import (
	"context"
	"os"

	beamlit "github.com/beamlit/toolkit/sdk"
)

const (
	envBaseURL     = "BEAMLIT_BASE_URL"
	envToken       = "BEAMLIT_TOKEN"
	defaultBaseURL = "https://api.beamlit.com/v0"
)

type Client struct {
	baseURL string
	client  *beamlit.Client
}

func NewClient() (*Client, error) {
	beamlitToken, err := NewBeamlitToken()
	if err != nil {
		return nil, err
	}

	baseURL := os.Getenv(envBaseURL)
	if baseURL == "" {
		baseURL = defaultBaseURL
	}

	client, err := beamlit.NewClient(baseURL, "", beamlit.WithHTTPClient(beamlitToken.client(context.Background())))
	if err != nil {
		return nil, err
	}
	return &Client{
		baseURL: baseURL,
		client:  client,
	}, nil
}
