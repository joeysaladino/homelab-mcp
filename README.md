# homelab-mcp

`homelab-mcp` is a self-hosted MCP gateway for carefully designed tools that
operate on services in a homelab. Mealie is the first integration; the project
is intended to grow into a small, maintainable platform for additional
services.

## First vertical slice

The initial milestone is intentionally narrow:

- typed configuration for the Mealie URL and server-side token;
- versioned, human-editable pantry configuration with inventory states;
- versioned, human-editable shopping-trip configuration;
- a typed Mealie API client with read operations and additive URL import;
- curated recipe, meal-plan, shopping-list, and grocery-draft MCP tools,
  including read-only search/detail/inspection, additive recipe import and
  meal-plan entry creation, and a confirmation-gated grocery-list apply tool;
- tests using an in-memory HTTP server and MCP transport.

Destructive operations, authentication, and observability will be added in
later slices. Grocery semantics are intentionally split: the server gathers
recipe and pantry context, while the LLM proposes the human-friendly merged
items and the server validates and writes them only after confirmation.

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
The server validates this file during startup. The read-only draft tool
includes this pantry context. The server does not silently subtract or infer
grocery quantities; the LLM can use pantry state when preparing final shopping
text, including `check pantry` or low-stock guidance where useful.

The default shopping-trip file is `config/shopping.yaml`; override it with
`SHOPPING_FILE`. Trips are named weekday groups. Unassigned weekdays are
allowed and are reported by the grocery-draft workflow instead of being
silently assigned to a run.

## Grocery draft and confirmed apply

The intended grocery workflow is two-phase:

1. Call `mealie.prepare_shopping_draft` for a date range. It returns meals,
   human-readable recipe ingredient sources, configured shopping trips, and
   pantry context. Imported recipe text is preserved even when Mealie's
   structured ingredient fields are incomplete.
2. Have the LLM normalize and merge those sources into final store-facing
   items such as `Ground cumin — check pantry` or `Cod fillets — 1.5 lb`.
3. Call `mealie.apply_shopping_lists` with `confirm: false` to validate and
   preview the new lists. This call performs no Mealie write.
4. After the user explicitly approves the preview, call the same tool with
   `confirm: true`. It creates new Mealie lists and items using the complete
   human-readable display text. It never replaces, merges, or deletes existing
   lists or items.

The apply tool requires `confirm` on every call. Its default-safe value is
`false`; callers should only send `true` after explicit user approval. Because
Mealie does not provide a transaction across list creation and item creation,
an upstream failure after an earlier list succeeds may leave that earlier new
list in place. The tool reports this rather than attempting an unsafe automatic
rollback.

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

The read-only grocery-draft check combines the live meal plan and recipes with
the configured pantry and shopping-trip context:

```sh
set -a
. ./.env
set +a
GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerPrepareShoppingDraft -count=1
```

The confirmed-apply tool can be exercised safely through the real stdio server
with its no-write preview mode:

```sh
set -a
. ./.env
set +a
GOCACHE=/private/tmp/homelab-mcp-go-cache go test -tags=integration ./cmd/homelab-mcp -run TestStdioServerApplyShoppingListsPreview -count=1
```

This validates server startup, MCP registration, schema decoding, draft
normalization, and the `confirm: false` safety boundary without creating a
Mealie list or item.

The integration test is excluded from normal test runs so local development
does not depend on network access or a live homelab.

Static checks used before committing are:

```sh
gofmt -w $(rg --files -g '*.go')
go vet ./...
go vet -tags=integration ./...
```

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
