// Package mcp implements ncgo's minimal MCP stdio server.
package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
)

const protocolVersion = "2025-06-18"

type Server struct {
	NCGOVersion   string
	AssetsVersion string
	BuildVersion  string
	BuildTime     string
}

func New(ncgoVersion, assetsVersion string, buildInfo ...string) *Server {
	s := &Server{NCGOVersion: ncgoVersion, AssetsVersion: assetsVersion, BuildVersion: "unknown", BuildTime: "unknown"}
	if len(buildInfo) > 0 && buildInfo[0] != "" {
		s.BuildVersion = buildInfo[0]
	}
	if len(buildInfo) > 1 && buildInfo[1] != "" {
		s.BuildTime = buildInfo[1]
	}
	return s
}

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"inputSchema"`
}

func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	br := bufio.NewReader(in)
	for {
		body, err := readFrame(br)
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		var req rpcRequest
		if err := json.Unmarshal(body, &req); err != nil {
			continue
		}
		if len(req.ID) == 0 {
			continue
		}
		resp := s.handle(ctx, req)
		if err := writeFrame(out, resp); err != nil {
			return err
		}
	}
}

func (s *Server) handle(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		return ok(req.ID, map[string]any{
			"protocolVersion": protocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": "ncgo", "version": s.NCGOVersion},
		})
	case "tools/list":
		return ok(req.ID, map[string]any{"tools": s.tools()})
	case "tools/call":
		res, err := s.callTool(ctx, req.Params)
		if err != nil {
			return fail(req.ID, -32602, err.Error())
		}
		return ok(req.ID, res)
	default:
		return fail(req.ID, -32601, "method not found: "+req.Method)
	}
}

func ok(id json.RawMessage, result any) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Result: result}
}

func fail(id json.RawMessage, code int, msg string) rpcResponse {
	return rpcResponse{JSONRPC: "2.0", ID: id, Error: &rpcError{Code: code, Message: msg}}
}

// readFrame reads one newline-delimited JSON message from the MCP stdio
// transport: a single line terminated by '\n'. Blank lines are tolerated and
// skipped, and a final message without a trailing newline is still returned.
func readFrame(r *bufio.Reader) ([]byte, error) {
	for {
		line, err := r.ReadString('\n')
		if trimmed := strings.TrimRight(line, "\r\n"); trimmed != "" {
			return []byte(trimmed), nil
		}
		if err != nil {
			return nil, err
		}
	}
}

func writeFrame(w io.Writer, resp rpcResponse) error {
	body, err := json.Marshal(resp)
	if err != nil {
		return err
	}
	body = append(body, '\n')
	_, err = w.Write(body)
	return err
}

func EncodeMessage(v any) []byte {
	body, _ := json.Marshal(v)
	return append(body, '\n')
}

func DecodeResponses(data []byte) ([]rpcResponse, error) {
	br := bufio.NewReader(bytes.NewReader(data))
	var out []rpcResponse
	for {
		body, err := readFrame(br)
		if errors.Is(err, io.EOF) {
			return out, nil
		}
		if err != nil {
			return nil, err
		}
		var resp rpcResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return nil, err
		}
		out = append(out, resp)
	}
}
