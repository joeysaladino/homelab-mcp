package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/joeysaladino/homelab-mcp/internal/config"
	"github.com/joeysaladino/homelab-mcp/internal/mealie"
	"github.com/joeysaladino/homelab-mcp/internal/pantry"
	"github.com/joeysaladino/homelab-mcp/internal/server"
	shoppingconfig "github.com/joeysaladino/homelab-mcp/internal/shopping"
	mealietools "github.com/joeysaladino/homelab-mcp/internal/tools/mealie"
	shoppingtools "github.com/joeysaladino/homelab-mcp/internal/tools/shopping"
	"github.com/modelcontextprotocol/go-sdk/mcp"
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

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	mcpServer := server.New(
		mealietools.NewModule(mealieClient),
		shoppingtools.NewModule(shoppingPlanner),
	)
	if err := mcpServer.Run(ctx, &mcp.StdioTransport{}); err != nil && !errors.Is(err, context.Canceled) {
		slog.Error("run MCP server", "error", err)
		os.Exit(1)
	}
}
