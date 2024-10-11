package beamlit

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

const (
	envBaseURL     = "BEAMLIT_BASE_URL"
	envToken       = "BEAMLIT_TOKEN"
	defaultBaseURL = "https://api.beamlit.io"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
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

	return &Client{
		baseURL:    baseURL,
		httpClient: beamlitToken.client(context.Background()),
	}, nil
}

// doRequest is a helper function to make a request to the Beamlit API
// It returns the response from the Beamlit API
// It returns an error if the request fails
func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s", c.baseURL, path), body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/json")
	return c.httpClient.Do(req)
}
