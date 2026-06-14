# ── Stage 1: Build frontend ─────────────────────────────────────────────────
FROM node:26-alpine AS frontend-builder
WORKDIR /frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# ── Stage 2: Build backend ───────────────────────────────────────────────────
FROM golang:1.26-alpine AS backend-builder
WORKDIR /app
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /sempa ./cmd/server

# ── Stage 3: Final image ─────────────────────────────────────────────────────
FROM alpine:latest
RUN apk --no-cache add ca-certificates tzdata
WORKDIR /app
COPY --from=backend-builder /sempa ./sempa
COPY --from=frontend-builder /frontend/build ./frontend/build
RUN mkdir -p /data
EXPOSE 9001
VOLUME ["/data"]
CMD ["./sempa"]
