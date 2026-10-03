# homelab-mcp

`homelab-mcp` is a self-hosted MCP gateway for carefully designed tools that
operate on services in a homelab. Mealie is the first integration; the project
is intended to grow into a small, maintainable platform for additional
services.

## First vertical slice

The initial milestone is intentionally narrow:

- typed configuration for the Mealie URL and server-side token;
- versioned, human-editable pantry configuration with inventory states;
- a typed Mealie API client with read operations and additive URL import;
- curated recipe, meal-plan, and shopping-list MCP tools, including read-only
  search/detail/inspection plus additive recipe import and meal-plan entry
  creation;
- tests using an in-memory HTTP server and MCP transport.

Destructive operations, shopping-list synthesis, authentication, and
observability will be added in later slices.

Integration tools are registered as modules. The application server accepts
generic modules, while each service owns its tool registration; adding a
future Vikunja module will not require expanding the server constructor.

## Local development

Keep local credentials in an ignored `.env` file, then export them only to the
process you launch:

```sh
set -a
. ./.env
set +a
go run ./cmd/homelab-mcp
```

The initial server uses MCP stdio transport. Protocol traffic uses stdout;
application logs use stderr.

## Pantry context

The default pantry file is `config/pantry.yaml`. Override its location with
`PANTRY_FILE` when running in Docker or Kubernetes. Each item supports the
states `have`, `low`, `out`, and `unknown`; omitted state defaults to `have`.
The server validates this file during startup, but pantry data is not yet used
to generate shopping lists.

## Testing

Run the normal offline test suite with:

```sh
GOCACHE=/private/tmp/homelab-mcp-go-cache go test ./...
```

An opt-in read-only integration test launches the actual server as a
subprocess and calls the recipe tools against the configured Mealie instance:

```sh
set -a
. ./.env
set +a
GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerRecipeTools -count=1
```

The read-only meal-plan check uses the proof-of-concept date range:

```sh
set -a
. ./.env
set +a
GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerMealPlan -count=1
```

The read-only shopping-list check discovers the first live list and retrieves
its items:

```sh
set -a
. ./.env
set +a
GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerShoppingLists -count=1
```

The integration test is excluded from normal test runs so local development
does not depend on network access or a live homelab.

Shopping-list writes are also opt-in. This test creates one new list and one
item, then verifies the result. Choose unique test values and run it only when
you intend to create those records in Mealie:

```sh
set -a
. ./.env
set +a
HOMELAB_MCP_LIVE_SHOPPING_WRITE=1 \
HOMELAB_MCP_LIVE_SHOPPING_LIST_NAME="MCP test list 2026-10-03" \
HOMELAB_MCP_LIVE_SHOPPING_ITEM_DISPLAY="MCP test item" \
GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerCreateShoppingList -count=1
```

The URL-import integration test is a separate, explicit write check. It first
looks for the exact source URL and skips if the recipe already exists:

```sh
set -a
. ./.env
set +a
HOMELAB_MCP_LIVE_IMPORT=1 GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerImportRecipeURL -count=1
```

Run that command only when you intend to create the supplied recipe in Mealie.

Meal-plan writes can be exercised with explicit environment values. The test
checks for a matching entry first and skips duplicate writes:

```sh
set -a
. ./.env
set +a
HOMELAB_MCP_LIVE_PLAN_DATE=2026-10-03 \
HOMELAB_MCP_LIVE_PLAN_ENTRY_TYPE=breakfast \
HOMELAB_MCP_LIVE_PLAN_RECIPE_ID=recipe-uuid \
GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerCreateMealPlanEntry -count=1
```
