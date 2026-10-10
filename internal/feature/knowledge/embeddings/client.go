package embeddings

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"

	"svc-registry/internal/feature/knowledge"
)

type Client struct {
	api   openai.Client
	model string
	dims  int
	batch int
}

func New(baseURL, apiKey, model string, dims, batch int, hc *http.Client) *Client {
	opts := []option.RequestOption{
		option.WithBaseURL(strings.TrimRight(baseURL, "/") + "/"),
		option.WithHTTPClient(hc),
		option.WithMaxRetries(1),
	}
	if apiKey != "" {
		opts = append(opts, option.WithAPIKey(apiKey))
	} else {
		opts = append(opts, option.WithAPIKey("none"))
	}
	return &Client{api: openai.NewClient(opts...), model: model, dims: dims, batch: max(batch, 1)}
}

func (c *Client) Model() string { return c.model }

func (c *Client) Dimensions() int { return c.dims }

func (c *Client) Embed(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, 0, len(texts))
	for start := 0; start < len(texts); start += c.batch {
		part := texts[start:min(len(texts), start+c.batch)]
		res, err := c.api.Embeddings.New(ctx, openai.EmbeddingNewParams{
			Model: openai.EmbeddingModel(c.model),
			Input: openai.EmbeddingNewParamsInputUnion{OfArrayOfStrings: part},
		})
		if err != nil {
			return nil, fmt.Errorf("%w: %v", knowledge.ErrEmbeddingsUnavailable, err)
		}
		if len(res.Data) != len(part) {
			return nil, fmt.Errorf("%w: %d vectors for %d inputs", knowledge.ErrEmbeddingsUnavailable, len(res.Data), len(part))
		}
		vectors := make([][]float32, len(part))
		for _, d := range res.Data {
			if int(d.Index) < 0 || int(d.Index) >= len(part) || len(d.Embedding) != c.dims {
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
