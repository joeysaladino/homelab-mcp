// Package server assembles the application's MCP server and its integrations.
package server

import (
	"github.com/joeysaladino/homelab-mcp/internal/tools/mealie"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	Name    = "homelab-mcp"
	Version = "0.1.0"
)

// New creates the MCP server and registers the currently supported tools.
func New(recipeSearcher mealie.RecipeSearcher, recipeGetter mealie.RecipeGetter, recipeImporter mealie.RecipeImporter) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:    Name,
		Version: Version,
	}, nil)
	mealie.RegisterSearchRecipes(server, recipeSearcher)
	mealie.RegisterGetRecipe(server, recipeGetter)
	mealie.RegisterImportRecipeURL(server, recipeImporter)
	return server
}
