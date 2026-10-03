# syntax=docker/dockerfile:1

FROM golang:1.26-alpine AS build

WORKDIR /src

# Keep dependency download cached independently from application source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
	-buildvcs=false \
	-trimpath \
	-ldflags="-s -w" \
	-o /out/homelab-mcp \
	./cmd/homelab-mcp

FROM gcr.io/distroless/static-debian12:nonroot

WORKDIR /app

COPY --from=build /out/homelab-mcp /app/homelab-mcp
COPY --from=build /src/config /app/config

# Runtime overrides can mount a different config directory or set these paths.
ENV PANTRY_FILE=/app/config/pantry.yaml \
	SHOPPING_FILE=/app/config/shopping.yaml

USER nonroot:nonroot
ENTRYPOINT ["/app/homelab-mcp"]
