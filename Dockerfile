FROM node:24-alpine AS frontend-builder

WORKDIR /app/frontend

COPY frontend/package*.json ./

RUN npm install

COPY frontend/ .

RUN npm run build

FROM golang:1.24.4-alpine AS backend-builder

RUN apk add --no-cache gcc musl-dev

ENV CGO_ENABLED=1

WORKDIR /app

COPY backend/go.mod backend/go.sum ./
RUN go mod download

COPY backend .

RUN go build -o ai-ui ./cmd

FROM alpine AS prod

WORKDIR /app

COPY --from=backend-builder /app/ai-ui /app/ai-ui
COPY --from=frontend-builder /app/frontend/dist ./static

EXPOSE 8080

CMD ["./ai-ui"]
