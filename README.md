# homelab-mcp

`homelab-mcp` is a self-hosted MCP gateway for carefully designed tools that
operate on services in a homelab. Mealie is the first integration; the project
is intended to grow into a small, maintainable platform for additional
services.

## First vertical slice

The initial milestone is intentionally narrow:

- typed configuration for the Mealie URL and server-side token;
- a typed Mealie API client with read operations and additive URL import;
- curated recipe and meal-plan MCP tools, including read-only search/detail/
  inspection plus additive recipe import and meal-plan entry creation;
- tests using an in-memory HTTP server and MCP transport.

Destructive operations, pantry state, shopping-list synthesis, authentication,
and observability will be added in later slices.

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

The integration test is excluded from normal test runs so local development
does not depend on network access or a live homelab.

The URL-import integration test is a separate, explicit write check. It first
looks for the exact source URL and skips if the recipe already exists:

```sh
set -a
. ./.env
set +a
HOMELAB_MCP_LIVE_IMPORT=1 GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerImportRecipeURL -count=1
```

Run that command only when you intend to create the supplied recipe in Mealie.
