// Copyright 2026 the Daimon authors.
// SPDX-License-Identifier: Apache-2.0

// Package mcp implements a lightweight MCP client over stdio transport.
// It speaks JSON-RPC 2.0 directly, with no external dependencies.
package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"sync"
	"sync/atomic"

	"github.com/sonicboom15/daimon/internal/conversation"
)

// rpcMsg is the common envelope for JSON-RPC 2.0 messages.
type rpcMsg struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      *int64          `json:"id,omitempty"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Client is an MCP client connected to a stdio subprocess.
// Each MCP server process gets its own Client.
type Client struct {
	name    string
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	writeMu sync.Mutex
	nextID  atomic.Int64

	pendMu  sync.Mutex
	pending map[int64]chan rpcMsg
}

// NewStdioClient starts command[0] with command[1:] as an MCP server subprocess,
// performs the MCP initialization handshake, and returns a ready Client.
func NewStdioClient(ctx context.Context, name string, command []string) (*Client, error) {
	if len(command) == 0 {
		return nil, fmt.Errorf("mcp %s: empty command", name)
	}
	cmd := exec.CommandContext(ctx, command[0], command[1:]...)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp %s: stdin pipe: %w", name, err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("mcp %s: stdout pipe: %w", name, err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("mcp %s: start: %w", name, err)
	}

	c := &Client{
		name:    name,
		cmd:     cmd,
		stdin:   stdin,
		pending: make(map[int64]chan rpcMsg),
	}
	go c.readLoop(stdout)

	if err := c.initialize(ctx); err != nil {
		_ = cmd.Process.Kill()
		_ = cmd.Wait() // reap zombie and unblock readLoop goroutine
		return nil, fmt.Errorf("mcp %s: initialize: %w", name, err)
	}
	return c, nil
}

// readLoop reads newline-delimited JSON from the server and dispatches
// responses to waiting callers.
func (c *Client) readLoop(r io.Reader) {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 1<<20), 1<<20) // 1 MiB per line
	for scanner.Scan() {
		var msg rpcMsg
		if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
			slog.Warn("mcp: malformed JSON from server", "server", c.name, "err", err)
			continue
		}
		if msg.ID == nil {
			continue // server notification; we don't need it
		}
		c.pendMu.Lock()
		ch, ok := c.pending[*msg.ID]
		if ok {
			delete(c.pending, *msg.ID)
		}
		c.pendMu.Unlock()
		if ok {
			ch <- msg
		}
	}
}

func (c *Client) write(msg rpcMsg) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	_, err = fmt.Fprintf(c.stdin, "%s\n", data)
	return err
}

// call sends a JSON-RPC request and blocks until the response arrives.
func (c *Client) call(ctx context.Context, method string, params any) (json.RawMessage, error) {
	id := c.nextID.Add(1)
	paramsRaw, err := json.Marshal(params)
	if err != nil {
		return nil, err
	}

	ch := make(chan rpcMsg, 1)
	c.pendMu.Lock()
	c.pending[id] = ch
	c.pendMu.Unlock()

	if err := c.write(rpcMsg{JSONRPC: "2.0", ID: &id, Method: method, Params: paramsRaw}); err != nil {
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
		return nil, fmt.Errorf("write: %w", err)
	}

	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("%s: %s", method, resp.Error.Message)
		}
		return resp.Result, nil
	case <-ctx.Done():
		c.pendMu.Lock()
		delete(c.pending, id)
		c.pendMu.Unlock()
		return nil, ctx.Err()
	}
}

// notify sends a JSON-RPC notification (no response expected).
func (c *Client) notify(method string, params any) error {
	paramsRaw, _ := json.Marshal(params)
	return c.write(rpcMsg{JSONRPC: "2.0", Method: method, Params: paramsRaw})
}

func (c *Client) initialize(ctx context.Context) error {
	_, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": "2024-11-05",
		"capabilities":    map[string]any{},
		"clientInfo":      map[string]any{"name": "daimon", "version": "0.0.1"},
	})
	if err != nil {
		return err
	}
	return c.notify("notifications/initialized", nil)
}

// ListTools returns all tools advertised by the MCP server.
func (c *Client) ListTools(ctx context.Context) ([]conversation.Tool, error) {
	raw, err := c.call(ctx, "tools/list", map[string]any{})
	if err != nil {
		return nil, err
	}
	var result struct {
		Tools []struct {
			Name        string          `json:"name"`
			Description string          `json:"description"`
			InputSchema json.RawMessage `json:"inputSchema"`
		} `json:"tools"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("parse tools/list: %w", err)
	}
	tools := make([]conversation.Tool, len(result.Tools))
	for i, t := range result.Tools {
		tools[i] = conversation.Tool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}
	return tools, nil
}

// CallTool executes a named tool and returns the concatenated text output.
func (c *Client) CallTool(ctx context.Context, name string, input json.RawMessage) (string, error) {
	var args any
	if len(input) > 0 {
		_ = json.Unmarshal(input, &args)
	}
	if args == nil {
		args = map[string]any{}
	}

	raw, err := c.call(ctx, "tools/call", map[string]any{
		"name":      name,
		"arguments": args,
	})
	if err != nil {
		return "", err
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
		IsError bool `json:"isError"`
	}
	if err := json.Unmarshal(raw, &result); err != nil {
		return "", fmt.Errorf("parse tools/call: %w", err)
	}

	var out string
	for _, item := range result.Content {
		if item.Type == "text" {
			out += item.Text
		}
	}
	if result.IsError {
		return "", fmt.Errorf("tool error: %s", out)
	}
	return out, nil
}

// Name returns the configured server name.
func (c *Client) Name() string { return c.name }

// Close shuts down the MCP server subprocess.
func (c *Client) Close() {
	_ = c.stdin.Close()
	_ = c.cmd.Wait()
}
