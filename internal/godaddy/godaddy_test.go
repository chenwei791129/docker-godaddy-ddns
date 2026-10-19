package godaddy

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCredentialAuthorizationHeaders(t *testing.T) {
	tests := []struct {
		name       string
		credential Credential
		want       string
	}{
		{"personal access token", PersonalAccessToken("pat-123"), "Bearer pat-123"},
		{"classic developer key", ClassicDeveloperKey{Key: "key", Secret: "secret"}, "sso-key key:secret"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				got = r.Header.Get("Authorization")
				_, _ = io.WriteString(w, "[]")
			}))
			defer srv.Close()

			c := &Client{BaseURL: srv.URL, Credential: tt.credential}
			if _, err := c.GetRecords(context.Background(), "example.com", "A", "@"); err != nil {
				t.Fatalf("GetRecords: %v", err)
			}
			if got != tt.want {
				t.Errorf("Authorization = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetRecords(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.EscapedPath() != "/v1/domains/example.com/records/A/@" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
		_, _ = io.WriteString(w, `[{"data":"203.0.113.7","name":"@","ttl":600,"type":"A"}]`)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Credential: PersonalAccessToken("t")}
	records, err := c.GetRecords(context.Background(), "example.com", "A", "@")
	if err != nil {
		t.Fatalf("GetRecords: %v", err)
	}
	if len(records) != 1 || records[0].Data != "203.0.113.7" || records[0].TTL != 600 {
		t.Errorf("records = %+v", records)
	}
}

func TestReplaceRecordsSendsBody(t *testing.T) {
	var gotBody []Record
	var gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.EscapedPath() != "/v1/domains/example.com/records/A/home" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.EscapedPath())
		}
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("decode body: %v", err)
		}
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Credential: PersonalAccessToken("t")}
	err := c.ReplaceRecords(context.Background(), "example.com", "A", "home", []Record{{Data: "198.51.100.4", TTL: 600}})
	if err != nil {
		t.Fatalf("ReplaceRecords: %v", err)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q", gotContentType)
	}
	if len(gotBody) != 1 || gotBody[0] != (Record{Data: "198.51.100.4", TTL: 600}) {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestAPIError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"code":"ACCESS_DENIED","message":"Authenticated user is not allowed access"}`)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Credential: PersonalAccessToken("t")}
	_, err := c.GetRecords(context.Background(), "example.com", "A", "@")
	var apiErr *APIError
	if !errors.As(err, &apiErr) {
		t.Fatalf("error = %v, want *APIError", err)
	}
	if apiErr.StatusCode != http.StatusForbidden || apiErr.Code != "ACCESS_DENIED" {
		t.Errorf("APIError = %+v", apiErr)
	}
	want := "GoDaddy API returned HTTP 403: ACCESS_DENIED: Authenticated user is not allowed access"
	if err.Error() != want {
		t.Errorf("Error() = %q, want %q", err.Error(), want)
	}
}

func TestAPIErrorWithoutJSONBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "upstream unavailable", http.StatusBadGateway)
	}))
	defer srv.Close()

	c := &Client{BaseURL: srv.URL, Credential: PersonalAccessToken("t")}
	err := c.ReplaceRecords(context.Background(), "example.com", "A", "@", []Record{{Data: "198.51.100.4"}})
	if err == nil || err.Error() != "GoDaddy API returned HTTP 502" {
		t.Errorf("error = %v", err)
	}
}

func TestMissingCredential(t *testing.T) {
	c := &Client{BaseURL: "http://127.0.0.1:1"}
	if _, err := c.GetRecords(context.Background(), "example.com", "A", "@"); err == nil {
		t.Error("expected an error without a credential")
	}
}

func TestAPIErrorFormats(t *testing.T) {
	tests := []struct {
		err  APIError
		want string
	}{
		{APIError{StatusCode: 401, Code: "UNABLE_TO_AUTHENTICATE", Message: "Unauthorized"}, "GoDaddy API returned HTTP 401: UNABLE_TO_AUTHENTICATE: Unauthorized"},
		{APIError{StatusCode: 401, Message: "Unauthorized"}, "GoDaddy API returned HTTP 401: Unauthorized"},
		{APIError{StatusCode: 401, Code: "UNABLE_TO_AUTHENTICATE"}, "GoDaddy API returned HTTP 401: UNABLE_TO_AUTHENTICATE"},
		{APIError{StatusCode: 500}, "GoDaddy API returned HTTP 500"},
	}
	for _, tt := range tests {
		if got := tt.err.Error(); got != tt.want {
			t.Errorf("Error() = %q, want %q", got, tt.want)
		}
	}
}
