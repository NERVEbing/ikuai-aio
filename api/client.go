// Package api implements only the iKuai 4.x Bearer-authenticated REST API.
package api

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const (
	BasePath         = "/api/v4.0"
	maxResponseBytes = 16 << 20
	pageSize         = 500
	maxPages         = 2000
)

type Options struct {
	Address            string
	Token              string
	Timeout            time.Duration
	InsecureSkipVerify bool
	ReadRetries        int
}

// Client can be shared by collectors and jobs. Credentials never leave the
// configured origin: redirects are rejected, including same-origin redirects.
type Client struct {
	base    *url.URL
	token   string
	http    *http.Client
	retries int
}

func NewClient(o Options) (*Client, error) {
	u, err := url.Parse(o.Address)
	if err != nil || u == nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return nil, errors.New("address must be an http(s) URL without credentials, query or fragment")
	}
	token := strings.TrimSpace(o.Token)
	if token == "" || strings.ContainsAny(token, " \t\r\n") {
		return nil, errors.New("an iKuai 4.x API token is required")
	}
	if o.Timeout <= 0 {
		return nil, errors.New("HTTP timeout must be positive")
	}
	if o.ReadRetries < 0 || o.ReadRetries > 5 {
		return nil, errors.New("read retries must be between 0 and 5")
	}
	u.Path = strings.TrimRight(u.Path, "/") + BasePath
	u.RawPath = ""
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.TLSClientConfig = &tls.Config{MinVersion: tls.VersionTLS12, InsecureSkipVerify: o.InsecureSkipVerify} // Explicit opt-in for self-signed router certificates.
	return &Client{base: u, token: token, retries: o.ReadRetries, http: &http.Client{
		Transport: transport, Timeout: o.Timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

func (c *Client) Close() { c.http.CloseIdleConnections() }

type Error struct {
	Status  int
	Code    int
	Method  string
	Path    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("iKuai %s %s: HTTP %d, code %d: %s", e.Method, e.Path, e.Status, e.Code, e.Message)
}

type envelope struct {
	Code    *int            `json:"code"`
	Message string          `json:"message"`
	Results json.RawMessage `json:"results"`
	Details []struct {
		Field   string `json:"field"`
		Message string `json:"msg"`
	} `json:"details"`
}

func (c *Client) request(ctx context.Context, method, endpoint string, query url.Values, input any) (envelope, error) {
	var body []byte
	if input != nil {
		var err error
		body, err = json.Marshal(input)
		if err != nil {
			return envelope{}, fmt.Errorf("encode request: %w", err)
		}
	}
	// A single timeout budget includes backoff and every retry of this request.
	ctx, cancel := context.WithTimeout(ctx, c.http.Timeout)
	defer cancel()
	for attempt := 0; ; attempt++ {
		u := *c.base
		u.Path += endpoint
		u.RawQuery = query.Encode()
		req, err := http.NewRequestWithContext(ctx, method, u.String(), bytes.NewReader(body))
		if err != nil {
			return envelope{}, err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("Accept", "application/json")
		if input != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.http.Do(req)
		if err != nil {
			// Writes are never replayed: an ambiguous failure may have committed.
			if method == http.MethodGet && attempt < c.retries && ctx.Err() == nil {
				if err = pause(ctx, retryDelay(attempt, "")); err == nil {
					continue
				}
			}
			return envelope{}, fmt.Errorf("iKuai %s %s: %w", method, endpoint, err)
		}
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes+1))
		resp.Body.Close()
		if readErr != nil {
			return envelope{}, fmt.Errorf("read iKuai response: %w", readErr)
		}
		if len(raw) > maxResponseBytes {
			return envelope{}, errors.New("iKuai response exceeds 16 MiB")
		}
		if method == http.MethodGet && attempt < c.retries && (resp.StatusCode == 429 || resp.StatusCode >= 500) {
			if err = pause(ctx, retryDelay(attempt, resp.Header.Get("Retry-After"))); err != nil {
				return envelope{}, err
			}
			continue
		}
		if resp.StatusCode == http.StatusNoContent && method != http.MethodGet {
			return envelope{}, nil
		}
		var env envelope
		decodeErr := json.Unmarshal(normalizeNil(raw), &env)
		if resp.StatusCode < 200 || resp.StatusCode >= 300 || (env.Code != nil && *env.Code != 0) {
			message := env.Message
			if message == "" {
				message = http.StatusText(resp.StatusCode)
			}
			for _, detail := range env.Details {
				message += "; " + detail.Field + ": " + detail.Message
			}
			code := 0
			if env.Code != nil {
				code = *env.Code
			}
			return envelope{}, &Error{Status: resp.StatusCode, Code: code, Method: method, Path: endpoint, Message: strings.ReplaceAll(message, c.token, "[redacted]")}
		}
		if decodeErr != nil {
			return envelope{}, fmt.Errorf("iKuai %s: invalid JSON response: %w", endpoint, decodeErr)
		}
		if env.Message == "" || (env.Code == nil && !strings.EqualFold(env.Message, "success")) {
			return envelope{}, fmt.Errorf("iKuai %s: unrecognized v4 response envelope", endpoint)
		}
		return env, nil
	}
}

func (c *Client) get(ctx context.Context, endpoint string, query url.Values, output any) error {
	env, err := c.request(ctx, http.MethodGet, endpoint, query, nil)
	if err != nil {
		return err
	}
	if len(env.Results) == 0 || bytes.Equal(env.Results, []byte("null")) {
		return fmt.Errorf("iKuai %s: missing results", endpoint)
	}
	if err = json.Unmarshal(env.Results, output); err != nil {
		return fmt.Errorf("iKuai %s: decode results: %w", endpoint, err)
	}
	return nil
}

func retryDelay(attempt int, header string) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds >= 0 {
		return time.Duration(min(seconds, 30)) * time.Second
	}
	if deadline, err := http.ParseTime(header); err == nil {
		return min(max(time.Until(deadline), 0), 30*time.Second)
	}
	return (100 * time.Millisecond) << attempt
}

func pause(ctx context.Context, duration time.Duration) error {
	t := time.NewTimer(duration)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// Some 4.x firmware emits bare nil in value positions. Replace only tokens
// outside strings; names such as "vanilla" and string values "nil" are intact.
func normalizeNil(raw []byte) []byte {
	var out bytes.Buffer
	quoted, escaped := false, false
	for i := 0; i < len(raw); i++ {
		b := raw[i]
		if quoted {
			out.WriteByte(b)
			if escaped {
				escaped = false
			} else if b == '\\' {
				escaped = true
			} else if b == '"' {
				quoted = false
			}
			continue
		}
		if b == '"' {
			quoted = true
		}
		if b == 'n' && i+3 <= len(raw) && string(raw[i:i+3]) == "nil" && (i == 0 || strings.ContainsRune(":,[ \t\r\n", rune(raw[i-1]))) && (i+3 == len(raw) || strings.ContainsRune(",}] \t\r\n", rune(raw[i+3]))) {
			out.WriteString("null")
			i += 2
		} else {
			out.WriteByte(b)
		}
	}
	return out.Bytes()
}

func list[T any](ctx context.Context, c *Client, endpoint, rowsKey, totalKey string) ([]T, error) {
	var all []T
	var previous []byte
	for page := 1; page <= maxPages; page++ {
		query := url.Values{"page": {strconv.Itoa(page)}, "limit": {strconv.Itoa(pageSize)}, "order_by": {"id"}, "order": {"asc"}}
		var payload map[string]json.RawMessage
		if err := c.get(ctx, endpoint, query, &payload); err != nil {
			return nil, err
		}
		data, ok := payload[rowsKey]
		if !ok {
			return nil, fmt.Errorf("iKuai %s: missing %s", endpoint, rowsKey)
		}
		var rows []T
		var total *int
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, fmt.Errorf("iKuai %s: decode rows: %w", endpoint, err)
		}
		if err := json.Unmarshal(payload[totalKey], &total); err != nil || total == nil || *total < 0 {
			return nil, fmt.Errorf("iKuai %s: invalid %s", endpoint, totalKey)
		}
		if len(rows) > 0 && bytes.Equal(previous, data) {
			return nil, fmt.Errorf("iKuai %s: server repeated a page", endpoint)
		}
		all = append(all, rows...)
		if len(all) >= *total {
			return all, nil
		}
		if len(rows) == 0 {
			return nil, fmt.Errorf("iKuai %s: incomplete pagination", endpoint)
		}
		previous = append(previous[:0], data...)
	}
	return nil, fmt.Errorf("iKuai %s: pagination exceeded %d pages", endpoint, maxPages)
}
