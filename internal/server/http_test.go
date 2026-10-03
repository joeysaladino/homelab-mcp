package server_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/joeysaladino/homelab-mcp/internal/server"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestHTTPHandlerHealthEndpoints(t *testing.T) {
	handler := server.NewHTTPHandler(mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.1.0"}, nil))

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
	httpServer := httptest.NewServer(server.NewHTTPHandler(mcpServer))
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

type testHTTPModule struct{}

func (testHTTPModule) RegisterTools(mcpServer *mcp.Server) {
	mcp.AddTool(mcpServer, &mcp.Tool{Name: "test.ping", Description: "test tool"}, func(context.Context, *mcp.CallToolRequest, struct{}) (*mcp.CallToolResult, struct{}, error) {
		return nil, struct{}{}, nil
	})
}
