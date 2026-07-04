package executor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

// ErrUnsupportedLanguage is returned when no sandbox tier handles a language.
var ErrUnsupportedLanguage = errors.New("executor: unsupported language")

// ErrTierDisabled is returned when a language routes to a disabled tier.
var ErrTierDisabled = errors.New("executor: sandbox tier disabled")

// Client proxies execution requests to the sandbox containers.
type Client struct {
	// LightURL / HeavyURL are the base URLs of the two sandbox images.
	LightURL string
	HeavyURL string
	// LightEnabled / HeavyEnabled gate routing to each tier.
	LightEnabled bool
	HeavyEnabled bool
	// HTTP is the client used for upstream calls; defaults to a sane client.
	HTTP *http.Client
}

// NewClient builds a Client with default upstream URLs and timeout.
func NewClient() *Client {
	return &Client{
		LightURL:     "http://exec-light:8765",
		HeavyURL:     "http://exec-heavy:8765",
		LightEnabled: true,
		HeavyEnabled: false,
		HTTP:         &http.Client{Timeout: 310 * time.Second},
	}
}

// URLForLanguage resolves the sandbox base URL for a language, honoring the
// enable flags. It returns ErrUnsupportedLanguage or ErrTierDisabled on failure.
func (c *Client) URLForLanguage(language string) (string, error) {
	tier, ok := TierFor(language)
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrUnsupportedLanguage, language)
	}
	switch tier {
	case TierLight:
		if !c.LightEnabled {
			return "", fmt.Errorf("%w: light", ErrTierDisabled)
		}
		return c.LightURL, nil
	case TierHeavy:
		if !c.HeavyEnabled {
			return "", fmt.Errorf("%w: heavy", ErrTierDisabled)
		}
		return c.HeavyURL, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedLanguage, language)
	}
}

// Execute performs a synchronous execution against the appropriate sandbox.
func (c *Client) Execute(ctx context.Context, req ExecRequest) (ExecResponse, error) {
	base, err := c.URLForLanguage(req.Language)
	if err != nil {
		return ExecResponse{}, err
	}

	body, err := json.Marshal(req)
	if err != nil {
		return ExecResponse{}, fmt.Errorf("executor: marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, base+"/exec", bytes.NewReader(body))
	if err != nil {
		return ExecResponse{}, fmt.Errorf("executor: build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	client := c.HTTP
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Do(httpReq)
	if err != nil {
		return ExecResponse{}, fmt.Errorf("executor: upstream unavailable: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return ExecResponse{}, fmt.Errorf("executor: read response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return ExecResponse{}, fmt.Errorf("executor: upstream status %d: %s", resp.StatusCode, string(raw))
	}

	var out ExecResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return ExecResponse{}, fmt.Errorf("executor: decode response: %w", err)
	}
	return out, nil
}
