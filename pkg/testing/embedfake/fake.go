package embedfake

import (
	"encoding/json"
	"hash/fnv"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"unicode"
)

var synonyms = map[string]string{
	"откатить": "rollback", "откат": "rollback", "откатывать": "rollback", "revert": "rollback", "rollback": "rollback",
	"релиз": "release", "release": "release", "выкат": "deploy", "deploy": "deploy", "deployment": "deploy", "deployments": "deploy",
}

type Fake struct {
	URL    string
	Dims   int
	mu     sync.Mutex
	calls  map[string]int
	broken bool
}

func Start(t *testing.T, dims int) *Fake {
	t.Helper()
	f := &Fake{Dims: dims, calls: map[string]int{}}
	srv := httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(srv.Close)
	f.URL = srv.URL + "/v1"
	return f
}

func (f *Fake) Break(on bool) {
	f.mu.Lock()
	f.broken = on
	f.mu.Unlock()
}

func (f *Fake) Calls(text string) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.calls[text]
}

func (f *Fake) Total() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, c := range f.calls {
		n += c
	}
	return n
}

func (f *Fake) serve(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	broken := f.broken
	f.mu.Unlock()
	if broken || r.URL.Path != "/v1/embeddings" {
		http.Error(w, `{"error":{"message":"unavailable"}}`, http.StatusServiceUnavailable)
		return
	}
	var body struct {
		Input []string `json:"input"`
		Model string   `json:"model"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	type item struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	}
	data := make([]item, len(body.Input))
	f.mu.Lock()
	for i, text := range body.Input {
		f.calls[text]++
		data[i] = item{Object: "embedding", Index: i, Embedding: Vector(text, f.Dims)}
	}
	f.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"object": "list", "model": body.Model, "data": data,
		"usage": map[string]int{"prompt_tokens": 1, "total_tokens": 1}})
}

func Vector(text string, dims int) []float64 {
	v := make([]float64, dims)
	for _, w := range strings.FieldsFunc(strings.ToLower(text), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsDigit(r) }) {
		if s, ok := synonyms[w]; ok {
			w = s
		}
		h := fnv.New32a()
		_, _ = h.Write([]byte(w))
		v[int(h.Sum32())%dims]++
	}
	var norm float64
	for _, x := range v {
		norm += x * x
	}
	if norm == 0 {
		v[0] = 1
		return v
	}
	norm = math.Sqrt(norm)
	for i := range v {
		v[i] /= norm
	}
	return v
}
