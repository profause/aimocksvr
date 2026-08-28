FROM node:22-alpine AS frontend
WORKDIR /app
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM golang:1.26-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /app/dist internal/web/dist/
# Ship a base config so Viper has full known keys (env vars still override
# every value at runtime). configs/config.yaml is gitignored and excluded by
# .dockerignore, so use the sanitized example as the in-image base config.
COPY configs/config.example.yaml configs/config.yaml
# Build the server and a static healthcheck binary. distroless ships no shell
# or network tools, so the healthcheck is a small Go binary copied into the
# final image.
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server ./cmd/server \
 && CGO_ENABLED=0 go build -ldflags="-s -w" -o /healthcheck ./cmd/healthcheck

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=builder /server /server
COPY --from=builder /healthcheck /healthcheck
# Viper resolves configs/ and ./ relative to the process working directory.
COPY --from=builder /src/configs/config.yaml /app/configs/config.yaml
EXPOSE 8080
ENTRYPOINT ["/server"]
