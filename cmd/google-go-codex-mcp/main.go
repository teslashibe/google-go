// google-go-codex-mcp exposes the google-go MCP provider over stdio for Codex.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	google "github.com/teslashibe/google-go"
	googlemcp "github.com/teslashibe/google-go/mcp"
	"github.com/teslashibe/mcptool"
)

const protocolVersion = "2025-03-26"

type request struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type toolCallParams struct {
	Name      string         `json:"name"`
	Arguments map[string]any `json:"arguments"`
}

type response struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func main() {
	configPath := flag.String("config", google.DefaultConfigPath(), "path to google-go config.json")
	flag.Parse()

	mgr, err := google.NewManager(*configPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "google-go-codex-mcp:", err)
		os.Exit(1)
	}

	tools := googlemcp.Provider{}.Tools()
	byName := make(map[string]mcptool.Tool, len(tools))
	for _, tool := range tools {
		byName[tool.Name] = tool
	}
	if err := serve(context.Background(), os.Stdin, os.Stdout, mgr, tools, byName); err != nil {
		fmt.Fprintln(os.Stderr, "google-go-codex-mcp:", err)
		os.Exit(1)
	}
}

func serve(ctx context.Context, in io.Reader, out io.Writer, mgr *google.Manager, tools []mcptool.Tool, byName map[string]mcptool.Tool) error {
	decoder := json.NewDecoder(bufio.NewReader(in))
	encoder := json.NewEncoder(out)
	for {
		var req request
		if err := decoder.Decode(&req); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return err
		}
		if len(req.ID) == 0 || string(req.ID) == "null" {
			continue
		}
		if err := encoder.Encode(handle(ctx, req, mgr, tools, byName)); err != nil {
			return err
		}
	}
}

func handle(ctx context.Context, req request, mgr *google.Manager, tools []mcptool.Tool, byName map[string]mcptool.Tool) response {
	respond := func(result any) response { return response{JSONRPC: "2.0", ID: req.ID, Result: result} }
	fail := func(code int, message string) response {
		return response{JSONRPC: "2.0", ID: req.ID, Error: &rpcError{Code: code, Message: message}}
	}

	switch req.Method {
	case "initialize":
		return respond(map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]string{"name": "google-go", "version": "dev"},
		})
	case "ping":
		return respond(map[string]any{})
	case "tools/list":
		listed := make([]map[string]any, 0, len(tools))
		for _, tool := range tools {
			listed = append(listed, map[string]any{
				"name": tool.Name, "description": tool.Description, "inputSchema": tool.InputSchema,
			})
		}
		return respond(map[string]any{"tools": listed})
	case "tools/call":
		var params toolCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return fail(-32602, "invalid tools/call parameters")
		}
		tool, ok := byName[params.Name]
		if !ok {
			return respond(toolError("unknown tool: " + params.Name))
		}
		raw, err := json.Marshal(params.Arguments)
		if err != nil {
			return fail(-32602, "invalid tool arguments")
		}
		value, err := tool.Invoke(ctx, mgr, raw)
		if err != nil {
			return respond(toolError(err.Error()))
		}
		body, err := json.Marshal(value)
		if err != nil {
			return respond(toolError(err.Error()))
		}
		return respond(map[string]any{"content": []map[string]string{{"type": "text", "text": string(body)}}})
	default:
		return fail(-32601, "method not found")
	}
}

func toolError(message string) map[string]any {
	return map[string]any{
		"content": []map[string]string{{"type": "text", "text": message}},
		"isError": true,
	}
}
