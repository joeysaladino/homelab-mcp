package mealie

import "github.com/modelcontextprotocol/go-sdk/mcp"

// Service is the complete capability set currently needed by the Mealie MCP
// tools. The individual tool interfaces remain small; this aggregate exists
// only at the integration boundary so server wiring stays stable as tools are
// added.
type Service interface {
	RecipeSearcher
	RecipeGetter
	RecipeImporter
	MealPlanReader
	MealPlanWriter
	ShoppingListLister
	ShoppingListGetter
}

// Module registers all Mealie-backed MCP tools.
type Module struct {
	service Service
}

// NewModule constructs the Mealie integration module from its service
// capabilities.
func NewModule(service Service) *Module {
	if service == nil {
		panic("create mealie module: nil service")
	}
	return &Module{service: service}
}

// RegisterTools implements server.Module without creating an import cycle.
func (m *Module) RegisterTools(server *mcp.Server) {
	if m == nil {
		panic("register mealie module: nil module")
	}
	if server == nil {
		panic("register mealie module: nil server")
	}

	RegisterSearchRecipes(server, m.service)
	RegisterGetRecipe(server, m.service)
	RegisterImportRecipeURL(server, m.service)
	RegisterGetMealPlan(server, m.service)
	RegisterCreateMealPlanEntry(server, m.service)
	RegisterGetShoppingLists(server, m.service)
	RegisterGetShoppingList(server, m.service)
}
