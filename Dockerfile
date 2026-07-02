FROM node:26-slim AS frontend-builder

WORKDIR /app

COPY package.json package-lock.json ./
COPY frontend/package.json ./frontend/package.json

RUN npm ci --include=optional --no-audit --no-fund

COPY frontend/ ./frontend

RUN cd frontend && npm run build

FROM golang:1.26.4-alpine AS backend-builder

WORKDIR /app

RUN apk add --no-cache gcc musl-dev

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend .

RUN CGO_ENABLED=1 go build -tags musl -ldflags="-s -w" -o ai-ui ./cmd

FROM alpine AS prod

WORKDIR /app

COPY --from=backend-builder /app/ai-ui /app/ai-ui
COPY --from=frontend-builder /app/frontend/dist ./static

EXPOSE 8080

CMD ["./ai-ui"]
