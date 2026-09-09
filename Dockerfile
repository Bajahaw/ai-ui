FROM node:26-slim AS frontend-builder

WORKDIR /app

COPY package.json package-lock.json ./
COPY frontend/package.json ./frontend/package.json

RUN npm ci --include=optional --no-audit --no-fund

COPY frontend/ ./frontend

ARG BUILD_ID
RUN BUILD_ID="${BUILD_ID:-$(date -u +%y%m%d.%H%M%S)}" \
    && printf '%s' "$BUILD_ID" > /build-id \
    && cd frontend \
    && VITE_BUILD_ID="$BUILD_ID" npm run build

FROM golang:1.26.4-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod \
    go mod download

COPY backend .
COPY --from=frontend-builder /build-id /build-id

RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    BUILD_ID="$(cat /build-id)" \
    && CGO_ENABLED=1 go build -tags musl \
      -ldflags="-s -w -X github.com/Bajahaw/ai-ui/cmd/version.BuildID=${BUILD_ID}" \
      -o ai-ui ./cmd

FROM alpine AS prod

WORKDIR /app

COPY --from=backend-builder /app/ai-ui /app/ai-ui
COPY --from=frontend-builder /app/frontend/dist ./static

EXPOSE 8080

CMD ["./ai-ui"]
