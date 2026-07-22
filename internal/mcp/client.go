package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/borch-ai/pithos/internal/config"
	mcpsdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

// PluginType represents the specific type/identity of the Powerword MCP plugin.
type PluginType string

const (
	// PluginImageGen represents the image generation plugin.
	PluginImageGen PluginType = "pw-mcp-imagegen"
	// PluginKDPMath represents the KDP mathematical layout plugin.
	PluginKDPMath PluginType = "pw-mcp-kdp-math"
	// PluginSEO represents the Amazon SEO/metadata plugin.
	PluginSEO PluginType = "pw-mcp-seo"
	// PluginViral represents the viral promotional/video plugin.
	PluginViral PluginType = "pw-mcp-viral"
	// PluginTypst represents the Typst compilation plugin.
	PluginTypst PluginType = "pw-mcp-typst"
	// PluginCloud represents the cloud storage/orchestrator plugin.
	PluginCloud PluginType = "pw-mcp-cloud"
	// PluginPDFCheck represents the PDF preflight validation plugin.
	PluginPDFCheck PluginType = "pw-mcp-pdfcheck"
)

// PluginClient handles connection lifecycle and requests to a specific MCP server.
type PluginClient struct {
	pluginType PluginType
	binaryPath string
	client     *mcpsdk.Client
	session    *mcpsdk.ClientSession
	transport  mcpsdk.Transport // Injected for unit testing
	mu         sync.Mutex
	env        []string
}

// NewPluginClient creates a new PluginClient using the default paths from config.
func NewPluginClient(pluginType PluginType) *PluginClient {
	return &PluginClient{
		pluginType: pluginType,
	}
}

// NewPluginClientWithBinary creates a new PluginClient using an explicit binary path.
func NewPluginClientWithBinary(pluginType PluginType, binaryPath string) *PluginClient {
	return &PluginClient{
		pluginType: pluginType,
		binaryPath: binaryPath,
	}
}

// SetTransport sets the transport for the plugin client (primarily for unit testing).
func (pc *PluginClient) SetTransport(t mcpsdk.Transport) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.transport = t
}

// SetEnv specifies custom environment variables to pass to the subprocess when started.
func (pc *PluginClient) SetEnv(env []string) {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	pc.env = env
}

// GetTransport returns the current transport (primarily for propagating mocks).
func (pc *PluginClient) GetTransport() mcpsdk.Transport {
	pc.mu.Lock()
	defer pc.mu.Unlock()
	return pc.transport
}

// ResolveBinaryPath determines the executable path to run for this plugin.
// Fallback path resolution prioritizes:
// 1. Explicitly configured binaryPath passed during client creation.
// 2. config.Cfg path if config is initialized.
// 3. Fallback to default plugin type string (e.g. system PATH resolution).
func (pc *PluginClient) ResolveBinaryPath() string {
	if pc.binaryPath != "" {
		return pc.binaryPath
	}
	if config.Cfg == nil {
		return string(pc.pluginType)
	}
	switch pc.pluginType {
	case PluginImageGen:
		return config.Cfg.MCP.ImageGenPath
	case PluginKDPMath:
		return config.Cfg.MCP.KDPMathPath
	case PluginSEO:
		return config.Cfg.MCP.SEOPath
	case PluginViral:
		return config.Cfg.MCP.ViralPath
	case PluginTypst:
		return config.Cfg.MCP.TypstPath
	case PluginCloud:
		return config.Cfg.MCP.CloudPath
	case PluginPDFCheck:
		return config.Cfg.MCP.PDFCheckPath
	default:
		return string(pc.pluginType)
	}
}

// Start launches the MCP server process and performs the protocol handshake.
func (pc *PluginClient) Start(ctx context.Context) error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.session != nil {
		return nil
	}

	binary := pc.ResolveBinaryPath()
	if binary == "" {
		return fmt.Errorf("no binary path configured or resolved")
	}

	transport := pc.transport
	if transport == nil {
		//nolint:gosec // G204: Subprocess launched with variable path from config fallback
		cmd := exec.CommandContext(ctx, binary)
		if len(pc.env) > 0 {
			cmd.Env = append(os.Environ(), pc.env...)
		}
		transport = &mcpsdk.CommandTransport{
			Command: cmd,
		}
	}

	pc.client = mcpsdk.NewClient(&mcpsdk.Implementation{
		Name:    "pithos",
		Version: config.Version,
	}, nil)

	session, err := pc.client.Connect(ctx, transport, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to MCP server %s: %w", binary, err)
	}

	pc.session = session
	return nil
}

// Stop cleanly terminates the connection and kills the child process.
func (pc *PluginClient) Stop() error {
	pc.mu.Lock()
	defer pc.mu.Unlock()

	if pc.session == nil {
		return nil
	}

	err := pc.session.Close()
	pc.session = nil
	pc.client = nil
	return err
}

// CallTool wraps the SDK's internal JSON-RPC tool calling capabilities.
// It parses the returned list of Content structures and concatenates TextContent.
func (pc *PluginClient) CallTool(ctx context.Context, toolName string, args map[string]interface{}) (string, error) {
	pc.mu.Lock()
	session := pc.session
	pc.mu.Unlock()

	if session == nil {
		return "", fmt.Errorf("plugin client not started")
	}

	params := &mcpsdk.CallToolParams{
		Name:      toolName,
		Arguments: args,
	}

	res, err := session.CallTool(ctx, params)
	if err != nil {
		return "", fmt.Errorf("calling tool %s: %w", toolName, err)
	}

	var sb strings.Builder
	for _, content := range res.Content {
		switch c := content.(type) {
		case *mcpsdk.TextContent:
			sb.WriteString(c.Text)
		default:
			data, err := json.Marshal(c)
			if err != nil {
				return "", fmt.Errorf("failed to marshal non-text tool content: %w", err)
			}
			sb.Write(data)
		}
	}

	if res.IsError {
		return sb.String(), fmt.Errorf("tool execution failed: %s", sb.String())
	}

	return sb.String(), nil
}
