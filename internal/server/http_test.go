package server_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/joeysaladino/homelab-mcp/internal/server"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func TestHTTPHandlerHealthEndpoints(t *testing.T) {
	handler := server.NewHTTPHandler(mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.1.0"}, nil), server.HTTPOptions{})

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "health", method: http.MethodGet, path: "/healthz", wantStatus: http.StatusOK, wantBody: "ok\n"},
		{name: "readiness", method: http.MethodGet, path: "/readyz", wantStatus: http.StatusOK, wantBody: "ok\n"},
		{name: "head", method: http.MethodHead, path: "/healthz", wantStatus: http.StatusOK},
		{name: "wrong method", method: http.MethodPost, path: "/healthz", wantStatus: http.StatusMethodNotAllowed},
		{name: "unknown path", method: http.MethodGet, path: "/not-found", wantStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recording := httptest.NewRecorder()
			request := httptest.NewRequest(tt.method, tt.path, nil)
			handler.ServeHTTP(recording, request)
			if recording.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recording.Code, tt.wantStatus)
			}
			if tt.wantBody != "" && recording.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", recording.Body.String(), tt.wantBody)
			}
		})
	}
}

func TestHTTPHandlerServesMCP(t *testing.T) {
	mcpServer := server.New(testHTTPModule{})
	httpServer := httptest.NewServer(server.NewHTTPHandler(mcpServer, server.HTTPOptions{}))
	defer httpServer.Close()

	ctx := context.Background()
	client := mcp.NewClient(&mcp.Implementation{Name: "http-test-client", Version: "0.1.0"}, nil)
	session, err := client.Connect(ctx, &mcp.StreamableClientTransport{
		Endpoint:             httpServer.URL + server.MCPPath,
		DisableStandaloneSSE: true,
		MaxRetries:           -1,
	}, nil)
	if err != nil {
		t.Fatalf("client.Connect() error = %v", err)
	}
	defer func() { _ = session.Close() }()

	tools, err := session.ListTools(ctx, nil)
	if err != nil {
		t.Fatalf("session.ListTools() error = %v", err)
	}
	if len(tools.Tools) != 1 || tools.Tools[0].Name != "test.ping" {
		t.Fatalf("tools = %+v, want test.ping", tools.Tools)
	}
}

func TestHTTPHandlerAuthMiddlewareProtectsMCPOnly(t *testing.T) {
	metadataURL := "https://mcp.example.test/.well-known/oauth-protected-resource"
	authMiddleware := mcpauth.RequireBearerToken(func(context.Context, string, *http.Request) (*mcpauth.TokenInfo, error) {
		return &mcpauth.TokenInfo{
			Scopes:     []string{"mcp.read"},
			Expiration: time.Now().Add(time.Hour),
			UserID:     "test-user",
		}, nil
	}, &mcpauth.RequireBearerTokenOptions{
		ResourceMetadataURL: metadataURL,
		Scopes:              []string{"mcp.read"},
	})
	metadataHandler := mcpauth.ProtectedResourceMetadataHandler(&oauthex.ProtectedResourceMetadata{
		Resource:             "https://mcp.example.test/mcp",
		AuthorizationServers: []string{"https://sso.example.test/realms/home"},
		ScopesSupported:      []string{"mcp.read"},
		BearerMethodsSupported: []string{
			"header",
		},
	})
	handler := server.NewHTTPHandler(server.New(testHTTPModule{}), server.HTTPOptions{
		AuthMiddleware:            authMiddleware,
		ProtectedResourceMetadata: metadataHandler,
	})

	tests := []struct {
		name       string
		request    func() *http.Request
		wantStatus int
	}{
		{
			name: "MCP requires bearer token",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodPost, server.MCPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}`))
			},
			wantStatus: http.StatusUnauthorized,
		},
		{
			name: "MCP accepts valid bearer token",
			request: func() *http.Request {
				request := httptest.NewRequest(http.MethodPost, server.MCPPath, strings.NewReader(`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-03-26","capabilities":{},"clientInfo":{"name":"test","version":"0.1.0"}}}`))
				request.Header.Set("Authorization", "Bearer test-token")
				request.Header.Set("Content-Type", "application/json")
				request.Header.Set("Accept", "application/json, text/event-stream")
				return request
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "health stays public",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, "/healthz", nil)
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "metadata stays public",
			request: func() *http.Request {
				return httptest.NewRequest(http.MethodGet, server.ProtectedResourceMetadataPath, nil)
			},
			wantStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recording := httptest.NewRecorder()
			handler.ServeHTTP(recording, tt.request())
			if recording.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body = %q", recording.Code, tt.wantStatus, recording.Body.String())
			}
			if tt.name == "MCP requires bearer token" && !strings.Contains(recording.Header().Get("WWW-Authenticate"), `resource_metadata="`+metadataURL+`"`) {
				t.Errorf("WWW-Authenticate = %q, want resource metadata URL", recording.Header().Get("WWW-Authenticate"))
			}
			if tt.name == "metadata stays public" {
				var metadata oauthex.ProtectedResourceMetadata
				if err := json.Unmarshal(recording.Body.Bytes(), &metadata); err != nil {
					t.Fatalf("decode metadata: %v", err)
				}
				if metadata.Resource != "https://mcp.example.test/mcp" {
					t.Errorf("metadata resource = %q, want configured resource", metadata.Resource)
				}
			}
		})
	}
}

type testHTTPModule struct{}

func (testHTTPModule) RegisterTools(mcpServer *mcp.Server) {
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "test.ping", Description: "test tool"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})
}
