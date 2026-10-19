// Package godaddy is a minimal client for the GoDaddy Domains API v1 DNS
// record endpoints. It is the only code that knows the API shape, so moving to
// another API version only changes this package.
package godaddy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

// DefaultBaseURL is the production GoDaddy API.
const DefaultBaseURL = "https://api.godaddy.com"

// Credential authenticates requests to the GoDaddy API.
type Credential interface {
	authorization() string
}

// PersonalAccessToken is GoDaddy's current scoped credential.
type PersonalAccessToken string

func (t PersonalAccessToken) authorization() string { return "Bearer " + string(t) }

// ClassicDeveloperKey is GoDaddy's legacy key and secret pair.
type ClassicDeveloperKey struct {
	Key    string
	Secret string
}

func (k ClassicDeveloperKey) authorization() string {
	return "sso-key " + k.Key + ":" + k.Secret
}

// Record is the data of one DNS record returned or sent for a type and name.
type Record struct {
	Data string `json:"data"`
	TTL  int    `json:"ttl,omitempty"`
}

// APIError is a non-2xx response from the GoDaddy API.
type APIError struct {
	StatusCode int
	Code       string
	Message    string
}

func (e *APIError) Error() string {
	msg := fmt.Sprintf("GoDaddy API returned HTTP %d", e.StatusCode)
	for _, detail := range []string{e.Code, e.Message} {
		if detail != "" {
			msg += ": " + detail
		}
	}
	return msg
}

// Client calls the GoDaddy DNS record endpoints.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
	Credential Credential
}

// GetRecords returns the records of the given type and name in domain.
func (c *Client) GetRecords(ctx context.Context, domain, recordType, name string) ([]Record, error) {
	var records []Record
	if err := c.do(ctx, http.MethodGet, recordsPath(domain, recordType, name), nil, &records); err != nil {
		return nil, err
	}
	return records, nil
}

// ReplaceRecords replaces all records of the given type and name in domain.
func (c *Client) ReplaceRecords(ctx context.Context, domain, recordType, name string, records []Record) error {
	body, err := json.Marshal(records)
	if err != nil {
		return err
	}
	return c.do(ctx, http.MethodPut, recordsPath(domain, recordType, name), body, nil)
}

func recordsPath(domain, recordType, name string) string {
	return "/v1/domains/" + url.PathEscape(domain) + "/records/" +
		url.PathEscape(recordType) + "/" + url.PathEscape(name)
}

func (c *Client) do(ctx context.Context, method, path string, body []byte, out any) error {
	if c.Credential == nil {
		return errors.New("no GoDaddy credential configured")
	}
	baseURL := c.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	req, err := http.NewRequestWithContext(ctx, method, strings.TrimRight(baseURL, "/")+path, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.Credential.authorization())
	req.Header.Set("Accept", "application/json")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	payload, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		apiErr := &APIError{StatusCode: resp.StatusCode}
		var envelope struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		}
		if json.Unmarshal(payload, &envelope) == nil {
			apiErr.Code, apiErr.Message = envelope.Code, envelope.Message
		}
		return apiErr
	}
	if out == nil || len(bytes.TrimSpace(payload)) == 0 {
		return nil
	}
	if err := json.Unmarshal(payload, out); err != nil {
		return fmt.Errorf("decode GoDaddy API response: %w", err)
	}
	return nil
}
