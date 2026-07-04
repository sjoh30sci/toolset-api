package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"
)

// searchUpstream is the base URL of the SearXNG service on the internal network.
// Overridable via SEARCH_UPSTREAM_URL (used by tests to point at a mock).
var searchUpstream = "http://search:8888"

// searchTimeout bounds a single upstream search request.
const searchTimeout = 5 * time.Second

// searchRequest is the inbound POST /search body.
type searchRequest struct {
	Query   string   `json:"query"`
	Engines []string `json:"engines,omitempty"`
	Page    int      `json:"page,omitempty"`
	Lang    string   `json:"lang,omitempty"`
}

// searchResult is a single normalized result in our response schema.
type searchResult struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Engine  string `json:"engine"`
}

// searchResponse is the outbound POST /search body.
type searchResponse struct {
	Query   string         `json:"query"`
	Page    int            `json:"page"`
	Results []searchResult `json:"results"`
	Count   int            `json:"count"`
}

// searxngResponse models the subset of SearXNG's JSON output we consume.
type searxngResponse struct {
	Results []struct {
		Title   string `json:"title"`
		URL     string `json:"url"`
		Content string `json:"content"`
		Engine  string `json:"engine"`
	} `json:"results"`
}

// Search handles POST /search by proxying to SearXNG and normalizing results.
func (h *Handlers) Search(c echo.Context) error {
	var req searchRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "invalid JSON body",
		})
	}
	if strings.TrimSpace(req.Query) == "" {
		return echo.NewHTTPError(http.StatusBadRequest, map[string]any{
			"error": "query is required",
		})
	}
	if req.Page <= 0 {
		req.Page = 1
	}

	// Build the SearXNG query string.
	q := url.Values{}
	q.Set("q", req.Query)
	q.Set("format", "json")
	q.Set("pageno", strconv.Itoa(req.Page))
	if len(req.Engines) > 0 {
		q.Set("engines", strings.Join(req.Engines, ","))
	}
	if req.Lang != "" {
		q.Set("language", req.Lang)
	}
	target := searchUpstream + "/search?" + q.Encode()

	ctx, cancel := context.WithTimeout(c.Request().Context(), searchTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, map[string]any{
			"error": "failed to build upstream request",
		})
	}
	httpReq.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, map[string]any{
			"error": fmt.Sprintf("search upstream unavailable: %v", err),
		})
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return echo.NewHTTPError(http.StatusBadGateway, map[string]any{
			"error": fmt.Sprintf("search upstream returned status %d", resp.StatusCode),
		})
	}

	var sx searxngResponse
	if err := json.NewDecoder(resp.Body).Decode(&sx); err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, map[string]any{
			"error": "failed to parse search upstream response",
		})
	}

	results := make([]searchResult, 0, len(sx.Results))
	for _, r := range sx.Results {
		results = append(results, searchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Content,
			Engine:  r.Engine,
		})
	}

	return c.JSON(http.StatusOK, searchResponse{
		Query:   req.Query,
		Page:    req.Page,
		Results: results,
		Count:   len(results),
	})
}
