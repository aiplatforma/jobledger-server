FROM docker.io/library/node:22-bookworm AS node-builder

WORKDIR /job-ledger

COPY internal/templates/*.html /job-ledger/internal/templates/
COPY internal/templates/*.css /job-ledger/internal/templates/
COPY internal/templates/package.json /job-ledger/internal/templates/
COPY internal/templates/pnpm-lock.yaml /job-ledger/internal/templates/
COPY internal/templates/pnpm-workspace.yaml /job-ledger/internal/templates/

RUN npm install -g pnpm

RUN --mount=type=cache,target=/root/.cache/pnpm \
    --mount=type=cache,target=/root/.local/share/pnpm \
    --mount=type=cache,target=/root/.pnpm-store \
    cd internal/templates && pnpm install --frozen-lockfile --prefer-offline --prod
RUN cd internal/templates && pnpm run css

FROM docker.io/library/golang:1.24-bookworm AS server-builder

WORKDIR /job-ledger

COPY go.mod go.sum /job-ledger/
RUN go mod download
COPY internal/ /job-ledger/internal/
COPY cmd/ /job-ledger/cmd/
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/root/.local/share/go \
    go build -o /job-ledger/job-ledger ./cmd/server/main.go

FROM docker.io/library/ubuntu:24.04

WORKDIR /app

RUN apt-get update && apt-get install -y ca-certificates && rm -rf /var/lib/apt/lists/*

COPY --from=server-builder /job-ledger/job-ledger /app/job-ledger
COPY --from=node-builder /job-ledger/static/ /app/static/
COPY internal/templates/*.html /app/internal/templates/
