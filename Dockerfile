# syntax=docker/dockerfile:1
FROM docker.io/library/node:24.22.2-bookworm-slim AS web
WORKDIR /src/web
COPY web/package*.json ./
RUN npm ci
COPY web/ ./
RUN npm run build

FROM docker.io/library/golang:1.27.3-bookworm AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=1 go build -trimpath -ldflags='-s -w' -o /out/panl .

FROM docker.io/library/debian:bookworm-slim
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates && rm -rf /var/lib/apt/lists/* \
    && mkdir -p /data /app/web && chown -R 10001:10001 /data /app
COPY --from=build /out/panl /usr/local/bin/panl
COPY --from=web /src/web/dist /app/web
USER 10001:10001
WORKDIR /app
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/panl", "--serve", "--listen=0.0.0.0:8080", "--db=/data/dashboards.db", "--web-dist=/app/web"]
CMD []
