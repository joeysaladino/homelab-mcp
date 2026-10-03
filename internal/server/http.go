package server

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// MCPPath is the stable HTTP path exposed to MCP clients and reverse proxies.
const MCPPath = "/mcp"

// ProtectedResourceMetadataPath is the RFC 9728 discovery endpoint used by
// OAuth-aware MCP clients after a 401 response.
const ProtectedResourceMetadataPath = "/.well-known/oauth-protected-resource"

// HTTPOptions supplies optional network-boundary middleware. Keeping the
// middleware as an http.Handler decorator lets the server package remain
// independent of a particular identity provider.
type HTTPOptions struct {
	AuthMiddleware            func(http.Handler) http.Handler
	ProtectedResourceMetadata http.Handler
}

// NewHTTPHandler builds the HTTP surface for the MCP server. Health endpoints
// are intentionally outside the MCP handler so Kubernetes probes do not need
// to understand JSON-RPC or establish an MCP session.
func NewHTTPHandler(mcpServer *mcp.Server, options HTTPOptions) http.Handler {
	if mcpServer == nil {
		panic("create HTTP handler: nil MCP server")
	}

	mcpHandler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcpServer
	}, nil)
	mcpEndpoint := http.Handler(mcpHandler)
	if options.AuthMiddleware != nil {
		mcpEndpoint = options.AuthMiddleware(mcpEndpoint)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", healthHandler)
	mux.HandleFunc("/readyz", readyHandler)
	mux.Handle(MCPPath, mcpEndpoint)
	mux.Handle(MCPPath+"/", mcpEndpoint)
	if options.ProtectedResourceMetadata != nil {
		mux.Handle(ProtectedResourceMetadataPath, options.ProtectedResourceMetadata)
	}
	return mux
}

// RunHTTP serves MCP over streamable HTTP until the context is canceled or
// the listener fails. It owns graceful shutdown so callers do not need to
// coordinate a separate http.Server.
func RunHTTP(ctx context.Context, addr string, mcpServer *mcp.Server, options HTTPOptions) error {
	if ctx == nil {
		return fmt.Errorf("run HTTP server: context is nil")
	}
	if mcpServer == nil {
		return fmt.Errorf("run HTTP server: MCP server is nil")
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return fmt.Errorf("run HTTP server: address is required")
	}

	httpServer := &http.Server{
		Addr:              addr,
		Handler:           NewHTTPHandler(mcpServer, options),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       2 * time.Minute,
	}
	serverErr := make(chan error, 1)
	go func() {
		slog.Info("MCP HTTP server listening", "addr", addr, "path", MCPPath)
		serverErr <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("run HTTP server: %w", err)
	case <-ctx.Done():
		shutdownContext, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(shutdownContext); err != nil {
			return fmt.Errorf("shutdown HTTP server: %w", err)
		}
		if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("run HTTP server: %w", err)
		}
		return nil
	}
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	writeHealthResponse(w, r)
}

func readyHandler(w http.ResponseWriter, r *http.Request) {
	writeHealthResponse(w, r)
}

func writeHealthResponse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", http.MethodGet+", "+http.MethodHead)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if r.Method == http.MethodGet {
		_, _ = io.WriteString(w, "ok\n")
	}
}
