package embeddings

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"svc-registry/internal/feature/knowledge"
)

const (
	retryPause    = 500 * time.Millisecond
	maxRetryAfter = 10 * time.Second
	errorBodyMax  = 512
)

type Client struct {
	url   string
	key   string
	model string
	dims  int
	batch int
	http  *http.Client
}

func New(baseURL, apiKey, model string, dims, batch int, hc *http.Client) *Client {
	if apiKey == "" {
		apiKey = "none"
	}
	return &Client{url: strings.TrimRight(baseURL, "/") + "/embeddings", key: apiKey, model: model, dims: dims,
		batch: max(batch, 1), http: hc}
}

func (c *Client) Model() string { return c.model }

func (c *Client) Dimensions() int { return c.dims }

type request struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type response struct {
	Data []struct {
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
}

type apiError struct {
	Error struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += c.batch {
		part := texts[start:min(len(texts), start+c.batch)]
		res, err := c.call(ctx, part)
		if err != nil {
			return nil, err
		}
		if len(res.Data) != len(part) {
			return nil, fmt.Errorf("%w: %d vectors for %d inputs", knowledge.ErrEmbeddingsUnavailable, len(res.Data), len(part))
		}
		vectors := make([][]float32, len(part))
		for _, d := range res.Data {
			if d.Index < 0 || d.Index >= len(part) || len(d.Embedding) != c.dims {
				return nil, fmt.Errorf("%w: vector %d has %d dimensions, want %d (EMBEDDINGS_DIMENSIONS)", knowledge.ErrEmbeddingsDimensions, d.Index, len(d.Embedding), c.dims)
			}
			v := make([]float32, len(d.Embedding))
			for i, x := range d.Embedding {
				v[i] = float32(x)
			}
			vectors[d.Index] = v
		}
		out = append(out, vectors...)
	}
	return out, nil
}

func (c *Client) call(ctx context.Context, input []string) (*response, error) {
	body, err := json.Marshal(request{Model: c.model, Input: input})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", knowledge.ErrEmbeddingsUnavailable, err)
	}
	res, retry, err := c.send(ctx, body)
	if retry > 0 {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("%w: %v", knowledge.ErrEmbeddingsUnavailable, ctx.Err())
		case <-time.After(retry):
		}
		res, _, err = c.send(ctx, body)
	}
	return res, err
}

func (c *Client) send(ctx context.Context, body []byte) (*response, time.Duration, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return nil, 0, fmt.Errorf("%w: %v", knowledge.ErrEmbeddingsUnavailable, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.key)
	r, err := c.http.Do(req)
	if err != nil {
		if ctx.Err() != nil {
			return nil, 0, fmt.Errorf("%w: %v", knowledge.ErrEmbeddingsUnavailable, err)
		}
		return nil, retryPause, fmt.Errorf("%w: %v", knowledge.ErrEmbeddingsUnavailable, err)
	}
	defer func() { _ = r.Body.Close() }()
	data, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, retryPause, fmt.Errorf("%w: read response: %v", knowledge.ErrEmbeddingsUnavailable, err)
	}
	if r.StatusCode < 200 || r.StatusCode > 299 {
		err := fmt.Errorf("%w: HTTP %d: %s", knowledge.ErrEmbeddingsUnavailable, r.StatusCode, errorText(data))
		return nil, retryAfter(r), err
	}
	var out response
	if err := json.Unmarshal(data, &out); err != nil {
		return nil, 0, fmt.Errorf("%w: invalid response: %v", knowledge.ErrEmbeddingsUnavailable, err)
	}
	return &out, 0, nil
}

func errorText(body []byte) string {
	var e apiError
	if json.Unmarshal(body, &e) == nil && e.Error.Message != "" {
		return e.Error.Message
	}
	text := strings.TrimSpace(string(body))
	if len(text) > errorBodyMax {
		text = text[:errorBodyMax] + "…"
	}
	return text
}

func retryAfter(r *http.Response) time.Duration {
	switch {
	case r.StatusCode == http.StatusRequestTimeout, r.StatusCode == http.StatusConflict,
		r.StatusCode == http.StatusTooManyRequests, r.StatusCode >= 500:
	default:
		return 0
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(r.Header.Get("Retry-After"))); err == nil && secs >= 0 {
		return max(min(time.Duration(secs)*time.Second, maxRetryAfter), time.Millisecond)
	}
	return retryPause
}
