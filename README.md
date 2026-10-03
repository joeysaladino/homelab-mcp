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
- an initial Keycloak-compatible OIDC bearer-token boundary for HTTP transport;
- tests using an in-memory HTTP server and MCP transport.

Destructive operations and observability will be added in later slices.
Authorization policy beyond configured scopes, audit logging, and richer
identity mapping are still future work. Grocery semantics are intentionally
split: the server gathers
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

For the network transport, set `MCP_TRANSPORT=http`. The server exposes the
streamable MCP endpoint at `/mcp` and Kubernetes-friendly probes at `/healthz`
and `/readyz`; `MCP_HTTP_ADDR` defaults to `:8080`.

HTTP mode defaults to OIDC authentication and fails closed when the required
OIDC settings are missing. Set `MCP_AUTH_MODE=none` only for isolated local
development. The current implementation validates bearer JWTs against the
configured issuer's discovery/JWKS endpoints, enforces the configured
audience, and can require space-separated scopes via `OIDC_REQUIRED_SCOPES`.

The server does not perform the client login flow or store user credentials;
an MCP client obtains an access token from Keycloak and sends it in the
`Authorization: Bearer` header. The protected-resource metadata endpoint is
available at `/.well-known/oauth-protected-resource` so OAuth-aware clients can
discover the configured authorization server.

For the initial Keycloak setup, use the realm issuer URL—not the Keycloak base
URL—as `OIDC_ISSUER_URL`, and set `OIDC_AUDIENCE` to the client identifier that
must appear in the access token's `aud` claim. If the access token only contains
the MCP client in `azp`, add a Keycloak audience protocol mapper so `aud`
contains the configured audience. Scope enforcement is optional until a scope
model is agreed; when enabled, the required values must be present in the
token's `scope` claim.

## Container image

The current transport is stdio, so the container is intended to be launched by
an MCP client as its stdio subprocess. Build the image with:

```sh
docker build -t homelab-mcp:dev .
```

Run it locally with credentials supplied at runtime and the editable config
directory mounted read-only:

```sh
set -a
. ./.env
set +a
docker run --rm -i \
  -e MEALIE_URL \
  -e MEALIE_TOKEN \
  -v "$PWD/config:/app/config:ro" \
  homelab-mcp:dev
```

The image runs as the unprivileged `nonroot` user. `.env` is excluded from the
build context and is never copied into the image. The example uses shell-style
environment files, so source the file and pass only the required variables to
Docker rather than using Docker's `--env-file` format. HTTP mode defaults to
OIDC; the explicit `MCP_AUTH_MODE=none` below is only for an isolated local
smoke test. HTTP mode exposes `/healthz` and `/readyz` for container-
orchestrator probes.

To run the container in HTTP mode locally:

```sh
set -a
. ./.env
set +a
docker run --rm -i \
  -e MEALIE_URL \
  -e MEALIE_TOKEN \
  -e MCP_TRANSPORT=http \
  -e MCP_AUTH_MODE=none \
  -e MCP_HTTP_ADDR=:8080 \
  -p 8080:8080 \
  -v "$PWD/config:/app/config:ro" \
  homelab-mcp:dev
```

Then verify the process without creating an MCP session:

```sh
curl --fail http://127.0.0.1:8080/healthz
curl --fail http://127.0.0.1:8080/readyz
```

## Kubernetes manifests

The starter manifests are in `deploy/kubernetes/base`. They run one
OIDC-protected HTTP-mode replica behind an internal `ClusterIP` Service, mount
the pantry and shopping configuration from an external ConfigMap, disable the
ServiceAccount token, and use the container's health/readiness endpoints for
probes.

Create the Mealie Secret separately; no credential values belong in Git:

```sh
set -a
. ./.env
set +a
kubectl create secret generic homelab-mcp-mealie \
  --from-literal=MEALIE_URL="$MEALIE_URL" \
  --from-literal=MEALIE_TOKEN="$MEALIE_TOKEN" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Create the OIDC runtime settings separately. These values are not necessarily
secret, but keeping them in a separately managed object avoids putting
environment-specific Keycloak and public-URL details in the base manifest:

```sh
kubectl create secret generic homelab-mcp-oidc \
  --from-literal=OIDC_ISSUER_URL="$OIDC_ISSUER_URL" \
  --from-literal=OIDC_AUDIENCE="$OIDC_AUDIENCE" \
  --from-literal=MCP_PUBLIC_URL="$MCP_PUBLIC_URL" \
  --from-literal=MCP_RESOURCE_METADATA_URL="$MCP_RESOURCE_METADATA_URL" \
  --dry-run=client -o yaml | kubectl apply -f -
```

Create or refresh the household ConfigMap from the repository's canonical
configuration files:

```sh
kubectl create configmap homelab-mcp-config \
  --from-file=pantry.yaml=config/pantry.yaml \
  --from-file=shopping.yaml=config/shopping.yaml \
  --dry-run=client -o yaml | kubectl apply -f -
```

Before applying, change the placeholder image `homelab-mcp:0.1.0` in the
manifest or provide a Kustomize overlay that points to the image registry used
by your cluster:

```sh
kubectl kustomize deploy/kubernetes/base
kubectl apply -k deploy/kubernetes/base
```

There is intentionally no Ingress in this base. Add one only after setting the
public URLs to the actual external route and confirming that the reverse proxy
preserves the `Authorization` header.

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
