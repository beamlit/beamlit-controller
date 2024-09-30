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
	token      string
	httpClient *http.Client
}

func NewClient() *Client {
	var baseURL, token string

	if baseURL = os.Getenv(envBaseURL); baseURL == "" {
		baseURL = defaultBaseURL
	}

	if token = os.Getenv(envToken); token == "" {
		token = ""
	}

	return &Client{
		baseURL:    baseURL,
		token:      token,
		httpClient: http.DefaultClient,
	}
}

func (c *Client) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s/%s", c.baseURL, path), body)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.token))
	req.Header.Add("Content-Type", "application/json")
	return c.httpClient.Do(req)
}
