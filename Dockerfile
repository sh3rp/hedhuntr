# syntax=docker/dockerfile:1

FROM node:22-bookworm AS web-build
WORKDIR /src
COPY package*.json ./
RUN npm ci
COPY index.html tsconfig.json vite.config.ts ./
COPY src ./src
COPY electron ./electron
RUN npm run build

FROM golang:1.22-bookworm AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN for cmd in api automation-worker description-fetcher matching-worker notification-worker parser-worker persistence-dispatcher resume-tuning-worker scheduler source-producer profile resume; do \
      go build -trimpath -ldflags="-s -w" -o "/out/${cmd}" "./cmd/${cmd}"; \
    done

FROM debian:bookworm-slim AS go-runtime
RUN apt-get update \
  && apt-get install -y --no-install-recommends ca-certificates tzdata \
  && rm -rf /var/lib/apt/lists/*
WORKDIR /app
COPY --from=go-build /out /app/bin
COPY configs /app/configs
COPY examples /app/examples
RUN mkdir -p /data/documents /data/automation-handoffs
VOLUME ["/data"]
ENTRYPOINT ["/app/bin/api"]
CMD ["-config", "/app/configs/deploy/api.json"]

FROM nginx:1.27-alpine AS web
COPY --from=web-build /src/dist /usr/share/nginx/html
COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
