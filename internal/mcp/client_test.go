package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/borch-ai/pithos/internal/config"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

var testImpl = &mcpsdk.Implementation{
	Name:    "test-impl",
	Version: "1.0.0",
}

func TestResolveBinaryPath(t *testing.T) {
	// 1. With explicit binaryPath
	pc := NewPluginClientWithBinary(PluginImageGen, "/path/to/explicit")
	if pc.resolveBinaryPath() != "/path/to/explicit" {
		t.Errorf("expected explicit path, got %q", pc.resolveBinaryPath())
	}

	// Save global config and restore after test
	origCfg := config.Cfg
	defer func() { config.Cfg = origCfg }()

	// 2. With nil config
	config.Cfg = nil
	pc2 := NewPluginClient(PluginKDPMath)
	if pc2.resolveBinaryPath() != string(PluginKDPMath) {
		t.Errorf("expected default name, got %q", pc2.resolveBinaryPath())
	}

	// 3. With config populated
	config.Cfg = &config.Config{
		MCP: config.MCPConfig{
			ImageGenPath: "/config/imagegen",
			KDPMathPath:  "/config/kdpmath",
			SEOPath:      "/config/seo",
			VideoPath:    "/config/video",
		},
	}

	tests := []struct {
		pType    PluginType
		expected string
	}{
		{PluginImageGen, "/config/imagegen"},
		{PluginKDPMath, "/config/kdpmath"},
		{PluginSEO, "/config/seo"},
		{PluginVideo, "/config/video"},
		{PluginType("unknown"), "unknown"},
	}

	for _, tt := range tests {
		pc := NewPluginClient(tt.pType)
		if pc.resolveBinaryPath() != tt.expected {
			t.Errorf("for %s: expected %q, got %q", tt.pType, tt.expected, pc.resolveBinaryPath())
		}
	}
}

func setupMockMCPServer(t *testing.T, ctx context.Context, serverTransport mcpsdk.Transport) (*mcpsdk.ServerSession, func()) {
	t.Helper()
	server := mcpsdk.NewServer(testImpl, nil)

	// Add a success tool
	server.AddTool(&mcpsdk.Tool{
		Name:        "greet",
		Description: "say hello",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		name := "world"
		if len(req.Params.Arguments) > 0 {
			var args struct {
				Name string `json:"name"`
			}
			if err := json.Unmarshal(req.Params.Arguments, &args); err == nil && args.Name != "" {
				name = args.Name
			}
		}
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "hello " + name},
			},
		}, nil
	})

	// Add an error tool
	server.AddTool(&mcpsdk.Tool{
		Name: "fail",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			IsError: true,
			Content: []mcpsdk.Content{
				&mcpsdk.TextContent{Text: "failed execution error"},
			},
		}, nil
	})

	// Add a non-text tool (returns image data)
	server.AddTool(&mcpsdk.Tool{
		Name: "image",
		InputSchema: map[string]any{
			"type": "object",
		},
	}, func(ctx context.Context, req *mcpsdk.CallToolRequest) (*mcpsdk.CallToolResult, error) {
		return &mcpsdk.CallToolResult{
			Content: []mcpsdk.Content{
				&mcpsdk.ImageContent{
					Data:     []byte("fakeimage"),
					MIMEType: "image/png",
				},
			},
		}, nil
	})

	serverSession, err := server.Connect(ctx, serverTransport, nil)
	if err != nil {
		t.Fatal(err)
	}

	return serverSession, func() {
		_ = serverSession.Close()
	}
}

func TestPluginClientLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	clientTransport, serverTransport := mcpsdk.NewInMemoryTransports()

	_, cleanup := setupMockMCPServer(t, ctx, serverTransport)
	defer cleanup()

	// 2. Setup PluginClient with clientTransport
	pc := NewPluginClient(PluginImageGen)
	pc.transport = clientTransport

	// Test calling tool before starting client
	_, err := pc.CallTool(ctx, "greet", nil)
	if err == nil || !strings.Contains(err.Error(), "plugin client not started") {
		t.Errorf("expected 'plugin client not started' error, got: %v", err)
	}

	// Start client
	err = pc.Start(ctx)
	if err != nil {
		t.Fatal(err)
	}

	// Test duplicate Start
	err = pc.Start(ctx)
	if err != nil {
		t.Errorf("expected duplicate Start to do nothing and return nil, got: %v", err)
	}

	// Call success tool
	res, err := pc.CallTool(ctx, "greet", map[string]any{"name": "tester"})
	if err != nil {
		t.Fatalf("CallTool greet failed: %v", err)
	}
	if res != "hello tester" {
		t.Errorf("expected 'hello tester', got %q", res)
	}

	// Call error tool
	resErr, err := pc.CallTool(ctx, "fail", nil)
	if err == nil {
		t.Error("expected error when tool returns IsError=true, got nil")
	} else if !strings.Contains(err.Error(), "failed execution error") {
		t.Errorf("expected error containing 'failed execution error', got: %v", err)
	}
	if resErr != "failed execution error" {
		t.Errorf("expected return string 'failed execution error', got %q", resErr)
	}

	// Call non-text tool (should fall back to JSON serialization)
	resImg, err := pc.CallTool(ctx, "image", nil)
	if err != nil {
		t.Fatalf("CallTool image failed: %v", err)
	}
	// Image content JSON should contain fields. Let's unmarshal or verify it starts with JSON.
	var imgWire struct {
		Type     string `json:"type"`
		MIMEType string `json:"mimeType"`
		Data     string `json:"data"`
	}
	if unmarshalErr := json.Unmarshal([]byte(resImg), &imgWire); unmarshalErr != nil {
		t.Errorf("expected JSON response for non-text content, got %q: %v", resImg, unmarshalErr)
	}
	if imgWire.MIMEType != "image/png" {
		t.Errorf("expected mimeType 'image/png', got %q", imgWire.MIMEType)
	}

	// Test non-existent tool call
	_, err = pc.CallTool(ctx, "nonexistent", nil)
	if err == nil {
		t.Error("expected error for nonexistent tool call, got nil")
	}

	// Stop client
	err = pc.Stop()
	if err != nil {
		t.Errorf("Stop failed: %v", err)
	}

	// Test duplicate Stop
	err = pc.Stop()
	if err != nil {
		t.Errorf("expected duplicate Stop to return nil, got: %v", err)
	}

	// Verify calling tool after Stop fails
	_, err = pc.CallTool(ctx, "greet", nil)
	if err == nil || !strings.Contains(err.Error(), "plugin client not started") {
		t.Errorf("expected error after Stop, got: %v", err)
	}
}
