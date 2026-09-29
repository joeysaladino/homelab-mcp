# homelab-mcp

`homelab-mcp` is a self-hosted MCP gateway for carefully designed tools that
operate on services in a homelab. Mealie is the first integration; the project
is intended to grow into a small, maintainable platform for additional
services.

## First vertical slice

The initial milestone is intentionally narrow:

- typed configuration for the Mealie URL and server-side token;
- a typed, read-only Mealie API client;
- two curated read-only MCP tools: `mealie.search_recipes` and
  `mealie.get_recipe`;
- tests using an in-memory HTTP server and MCP transport.

Write operations, pantry state, shopping-list synthesis, authentication, and
observability will be added in later slices.

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

An opt-in integration test launches the actual server as a subprocess and
calls `mealie.search_recipes` against the configured Mealie instance:

```sh
set -a
. ./.env
set +a
GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerRecipeTools -count=1
```

The integration test is excluded from normal test runs so local development
does not depend on network access or a live homelab.
