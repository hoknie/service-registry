package mcpfake

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

type Tool struct {
	Name         string
	Title        string
	Description  string
	InputSchema  any
	OutputSchema any
	ReadOnly     bool
	Answer       func(args json.RawMessage) *sdk.CallToolResult
}

type Call struct {
	Tool string
	Args json.RawMessage
	Auth string
}

type Server struct {
	URL          string
	SSE          string
	srv          *httptest.Server
	mu           sync.Mutex
	tools        []Tool
	header       string
	value        string
	status       int
	delay        time.Duration
	calls        []Call
	inits        int
	instructions string
	streamable   http.Handler
	favicon      []byte
}

func Start(t testing.TB, tools ...Tool) *Server {
	t.Helper()
	s := &Server{tools: tools, header: "Authorization"}
	s.reset()
	sse := sdk.NewSSEHandler(func(*http.Request) *sdk.Server { return s.server() }, nil)
	mux := http.NewServeMux()
	mux.Handle("/mcp", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.serve(w, r, s.current()) }))
	mux.Handle("/sse", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { s.serve(w, r, sse) }))
	mux.Handle("/favicon.ico", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		icon := s.favicon
		s.mu.Unlock()
		if icon == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		_, _ = w.Write(icon)
	}))
	s.srv = httptest.NewServer(mux)
	t.Cleanup(s.srv.Close)
	s.URL, s.SSE = s.srv.URL+"/mcp", s.srv.URL+"/sse"
	return s
}

func (s *Server) RequireHeader(name, value string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.header, s.value = name, value
}

func (s *Server) SetFavicon(png []byte) { s.mu.Lock(); s.favicon = png; s.mu.Unlock() }

func (s *Server) SetStatus(code int) { s.mu.Lock(); s.status = code; s.mu.Unlock() }

func (s *Server) SetDelay(d time.Duration) { s.mu.Lock(); s.delay = d; s.mu.Unlock() }

func (s *Server) SetTools(tools ...Tool) { s.mu.Lock(); s.tools = tools; s.mu.Unlock() }

func (s *Server) SetInstructions(text string) { s.mu.Lock(); s.instructions = text; s.mu.Unlock() }

func (s *Server) DropSessions() { s.reset() }

func (s *Server) Stop() { s.srv.Close() }

func (s *Server) Calls() []Call {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Call(nil), s.calls...)
}

func (s *Server) Initializations() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.inits
}

func (s *Server) reset() {
	h := sdk.NewStreamableHTTPHandler(func(*http.Request) *sdk.Server { return s.server() }, &sdk.StreamableHTTPOptions{JSONResponse: true})
	s.mu.Lock()
	s.streamable = h
	s.mu.Unlock()
}

func (s *Server) current() http.Handler {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.streamable
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request, next http.Handler) {
	s.mu.Lock()
	status, header, value, delay := s.status, s.header, s.value, s.delay
	s.mu.Unlock()
	if status != 0 {
		w.WriteHeader(status)
		return
	}
	if value != "" && r.Header.Get(header) != value {
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if delay > 0 && r.Method == http.MethodPost {
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
			return
		}
	}
	if r.Method == http.MethodPost && r.Body != nil {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte(`"method":"initialize"`)) {
			s.mu.Lock()
			s.inits++
			s.mu.Unlock()
		}
		r.Body = io.NopCloser(bytes.NewReader(body))
	}
	next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), authKey{}, r.Header.Get(header))))
}

type authKey struct{}

func (s *Server) server() *sdk.Server {
	s.mu.Lock()
	tools, instructions := append([]Tool(nil), s.tools...), s.instructions
	s.mu.Unlock()
	srv := sdk.NewServer(&sdk.Implementation{Name: "fake", Title: "Fake MCP", Version: "1.2.3"}, &sdk.ServerOptions{Instructions: instructions})
	for _, tool := range tools {
		input := tool.InputSchema
		if input == nil {
			input = map[string]any{"type": "object"}
		}
		def := &sdk.Tool{Name: tool.Name, Title: tool.Title, Description: tool.Description, InputSchema: input, OutputSchema: tool.OutputSchema}
		if tool.ReadOnly {
			def.Annotations = &sdk.ToolAnnotations{ReadOnlyHint: true}
		}
		srv.AddTool(def, s.handler(tool))
	}
	return srv
}

func (s *Server) handler(tool Tool) sdk.ToolHandler {
	return func(ctx context.Context, req *sdk.CallToolRequest) (*sdk.CallToolResult, error) {
		auth, _ := ctx.Value(authKey{}).(string)
		args := json.RawMessage(req.Params.Arguments)
		s.mu.Lock()
		s.calls = append(s.calls, Call{Tool: tool.Name, Args: append(json.RawMessage(nil), args...), Auth: auth})
		s.mu.Unlock()
		if tool.Answer != nil {
			return tool.Answer(args), nil
		}
		return &sdk.CallToolResult{Content: []sdk.Content{&sdk.TextContent{Text: tool.Name + ":" + string(args)}}}, nil
	}
}
