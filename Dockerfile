# syntax=docker/dockerfile:1

# --- Frontend build ---
FROM oven/bun:1 AS frontend
WORKDIR /frontend
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates git python3 make g++ \
    && curl -fsSL https://deb.nodesource.com/setup_22.x | bash - \
    && apt-get install -y --no-install-recommends nodejs \
    && rm -rf /var/lib/apt/lists/* \
    && ln -sf /usr/bin/python3 /usr/bin/python
COPY frontend/package.json frontend/bun.lock ./
# lefthook (prepare script) needs a git repo in cwd; we don't ship .git into
# the build context, so initialize a throwaway one for the install step.
RUN git init -q . && bun install --frozen-lockfile
COPY frontend ./
ENV NODE_OPTIONS="--max_old_space_size=16384"
ENV REACT_APP_CONSOLE_GIT_SHA=base58-fork
ENV REACT_APP_CONSOLE_GIT_REF=base58-fork
ENV REACT_APP_BUILD_TIMESTAMP=0
RUN bun run build

# --- Backend build ---
FROM golang:1.26-alpine AS backend
WORKDIR /app
RUN apk add --no-cache git
ENV GOTOOLCHAIN=auto

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend ./
COPY --from=frontend /frontend/build ./pkg/embed/frontend

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd cmd/api && CGO_ENABLED=0 GOOS=linux go build -o /console-api .

# --- Runtime ---
FROM alpine:3.21
RUN apk --no-cache add ca-certificates wget
WORKDIR /app
COPY --from=backend /console-api ./console-api
EXPOSE 8080
ENTRYPOINT ["/app/console-api"]
