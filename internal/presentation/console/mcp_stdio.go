package console

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/spf13/cobra"

	"svc-registry/internal/app"
	"svc-registry/internal/platform/config"
	"svc-registry/internal/platform/outbound"
)

func mcpStdioCmd() *cobra.Command {
	var address, tokenEnv string
	cmd := &cobra.Command{
		Use:   "mcp:stdio --url <address> --token-env <NAME>",
		Short: "Bridge MCP over stdio to the registry's /api/mcp",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return failed(mcpStdio(cmd, address, tokenEnv))
		},
	}
	cmd.Flags().StringVar(&address, "url", "", "address of the registry, e.g. https://registry.example")
	cmd.Flags().StringVar(&tokenEnv, "token-env", "", "name of the environment variable with a personal access token (scope mcp)")
	_ = cmd.MarkFlagRequired("url")
	_ = cmd.MarkFlagRequired("token-env")
	return cmd
}

const stdioMaxLine = 2 << 20

func mcpStdio(cmd *cobra.Command, address, tokenEnv string) error {
	env := config.ProcessEnv()
	logCfg, err := config.Load[config.LogConfig](env)
	if err != nil {
		return invalidConfig(err)
	}
	app.InitLogging(logCfg)
	out, err := config.Load[config.OutboundConfig](env)
	if err != nil {
		return invalidConfig(err)
	}
	token := strings.TrimSpace(env[tokenEnv])
	if token == "" {
		return fmt.Errorf("%s is not set: put a personal access token with the scope mcp there", tokenEnv)
	}
	u, err := url.Parse(strings.TrimSuffix(address, "/"))
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Host == "" {
		return fmt.Errorf("--url must be an http(s) address of the registry")
	}
	endpoint := u.String() + "/api/mcp"
	client := outbound.New(out)
	w := bufio.NewWriter(cmd.OutOrStdout())
	in := bufio.NewScanner(cmd.InOrStdin())
	in.Buffer(make([]byte, 64<<10), stdioMaxLine)
	for in.Scan() {
		line := bytes.TrimSpace(in.Bytes())
		if len(line) == 0 {
			continue
		}
		answer := forward(client, endpoint, token, line)
		if answer == nil {
			continue
		}
		if _, err := w.Write(append(answer, '\n')); err != nil {
			return err
		}
		if err := w.Flush(); err != nil {
			return err
		}
	}
	return in.Err()
}

func forward(client *http.Client, endpoint, token string, msg []byte) []byte {
	var head struct {
		ID json.RawMessage `json:"id"`
	}
	_ = json.Unmarshal(msg, &head)
	isRequest := len(head.ID) > 0 && string(head.ID) != "null"
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, bytes.NewReader(msg))
	if err != nil {
		return rpcError(head.ID, isRequest, "invalid registry address")
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := client.Do(req)
	if err != nil {
		var u *url.Error
		if errors.As(err, &u) {
			err = u.Err
		}
		return rpcError(head.ID, isRequest, "registry unreachable: "+err.Error())
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if err != nil {
		return rpcError(head.ID, isRequest, "registry answer not read")
	}
	if resp.StatusCode >= 300 {
		var apiErr struct {
			Code string `json:"code"`
		}
		_ = json.Unmarshal(body, &apiErr)
		return rpcError(head.ID, isRequest, fmt.Sprintf("registry refused the request: %d %s", resp.StatusCode, apiErr.Code))
	}
	if len(bytes.TrimSpace(body)) == 0 {
		return nil
	}
	var compact bytes.Buffer
	if err := json.Compact(&compact, body); err != nil {
		return rpcError(head.ID, isRequest, "registry answer is not JSON")
	}
	return compact.Bytes()
}

func rpcError(id json.RawMessage, isRequest bool, message string) []byte {
	if !isRequest {
		return nil
	}
	b, _ := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": id, "error": map[string]any{"code": -32000, "message": message}})
	return b
}
