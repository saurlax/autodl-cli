// Package api implements AutoDL's documented HTTP envelope protocol.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Response struct {
	Code      string          `json:"code"`
	Data      json.RawMessage `json:"data"`
	Message   string          `json:"msg"`
	RequestID string          `json:"request_id,omitempty"`
}

type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

func New(baseURL, token string, timeout time.Duration) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), Token: token, HTTP: &http.Client{
		Timeout: timeout,
		// Never forward a developer token to a redirect target.
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

func (c *Client) Call(ctx context.Context, method, path string, body any) (*Response, error) {
	encoded, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.BaseURL+path, bytes.NewReader(encoded))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", c.Token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		return nil, fmt.Errorf("request failed (check network connectivity or --timeout)")
	}
	defer resp.Body.Close()
	const maxBody = 8 << 20
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxBody+1))
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}
	if len(data) > maxBody {
		return nil, fmt.Errorf("API response exceeds 8 MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API returned HTTP %d", resp.StatusCode)
	}
	var envelope Response
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("API returned invalid JSON")
	}
	if envelope.Code != "Success" {
		// Defensively redact credentials if a server echoes them in an error.
		detail := envelope.Code + ": " + envelope.Message
		if c.Token != "" {
			detail = strings.ReplaceAll(detail, c.Token, "[REDACTED]")
		}
		return nil, fmt.Errorf("API error %s (request ID: %s)", detail, envelope.RequestID)
	}
	if len(envelope.Data) == 0 {
		envelope.Data = json.RawMessage("null")
	}
	return &envelope, nil
}
