# syntax=docker/dockerfile:1

# Build both binaries: the API and the migration runner (migrations are embedded).
FROM golang:1.27.2-alpine AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api && \
    CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/migrate ./cmd/migration

# Static binaries on distroless: no shell, no package manager, runs as uid 65532.
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/api /out/migrate /app/
# Mount martgo-auth's public key at /app/keys/jwt_public.pem (the default
# JWT_PUBLIC_KEY_FILE is relative to /app). Config comes from the environment.
ENV HTTP_ADDR=:8080
EXPOSE 8080
USER nonroot:nonroot
# Run migrations with: docker run --entrypoint /app/migrate <image> up
ENTRYPOINT ["/app/api"]
