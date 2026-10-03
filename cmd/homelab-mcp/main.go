package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	goauth "github.com/joeysaladino/homelab-mcp/internal/auth"
	"github.com/joeysaladino/homelab-mcp/internal/config"
	"github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/joeysaladino/homelab-mcp/internal/pantry"
	"github.com/joeysaladino/homelab-mcp/internal/server"
	shoppingconfig "github.com/joeysaladino/homelab-mcp/internal/shopping"
	mealietools "github.com/joeysaladino/homelab-mcp/internal/tools/mealie"
	shoppingtools "github.com/joeysaladino/homelab-mcp/internal/tools/shopping"
	mcpauth "github.com/modelcontextprotocol/go-sdk/auth"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/modelcontextprotocol/go-sdk/oauthex"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("load configuration", "error", err)
		os.Exit(1)
	}

	mealieClient, err := mealie.NewClient(cfg.Mealie, nil)
	if err != nil {
		slog.Error("create Mealie client", "error", err)
		os.Exit(1)
	}

	pantryContext, err := pantry.LoadFile(cfg.PantryFile)
	if err != nil {
		slog.Error("load pantry context", "error", err)
		os.Exit(1)
	}
	slog.Info("loaded pantry context", "path", cfg.PantryFile, "items", len(pantryContext.Items))

	shoppingPlan, err := shoppingconfig.LoadFile(cfg.ShoppingFile)
	if err != nil {
		slog.Error("load shopping-trip configuration", "error", err)
		os.Exit(1)
	}
	slog.Info("loaded shopping-trip configuration", "path", cfg.ShoppingFile, "trips", len(shoppingPlan.Trips))

	shoppingPlanner := shoppingconfig.NewPlanner(mealieClient, mealieClient, pantryContext, shoppingPlan)
	shoppingApplier := shoppingconfig.NewApplier(mealieClient, mealieClient)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpServer := server.New(
		mealietools.NewModule(mealieClient),
		shoppingtools.NewModule(shoppingPlanner, shoppingApplier),
	)

	var runErr error
	switch cfg.Transport {
	case config.TransportStdio:
		runErr = mcpServer.Run(ctx, &mcp.StdioTransport{})
	case config.TransportHTTP:
		httpOptions, err := buildHTTPOptions(ctx, cfg)
		if err != nil {
			runErr = fmt.Errorf("configure HTTP authentication: %w", err)
		} else {
			runErr = server.RunHTTP(ctx, cfg.HTTPAddr, mcpServer, httpOptions)
		}
	default:
		runErr = errors.New("unsupported MCP transport")
	}
	if runErr != nil && !errors.Is(runErr, context.Canceled) {
		slog.Error("run MCP server", "error", runErr)
		os.Exit(1)
	}
}

func buildHTTPOptions(ctx context.Context, cfg config.Config) (server.HTTPOptions, error) {
	if cfg.AuthMode == config.AuthNone {
		return server.HTTPOptions{}, nil
	}
	if cfg.AuthMode != config.AuthOIDC {
		return server.HTTPOptions{}, fmt.Errorf("unsupported authentication mode %q", cfg.AuthMode)
	}

	discoveryContext, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	verifier, err := goauth.NewOIDCVerifier(discoveryContext, goauth.OIDCConfig{
		IssuerURL: cfg.OIDCIssuerURL,
		Audience:  cfg.OIDCAudience,
	})
	if err != nil {
		return server.HTTPOptions{}, err
	}

	metadata := &oauthex.ProtectedResourceMetadata{
		Resource:             cfg.PublicURL,
		AuthorizationServers: []string{cfg.OIDCIssuerURL},
		ScopesSupported:      cfg.OIDCScopes,
		BearerMethodsSupported: []string{
			"header",
		},
	}
	return server.HTTPOptions{
		AuthMiddleware: mcpauth.RequireBearerToken(verifier.Verify, &mcpauth.RequireBearerTokenOptions{
			ResourceMetadataURL: cfg.MetadataURL,
			Scopes:              cfg.OIDCScopes,
		}),
		ProtectedResourceMetadata: mcpauth.ProtectedResourceMetadataHandler(metadata),
	}, nil
}
