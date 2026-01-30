# Build backend
FROM golang:1.24-alpine AS backend-builder
RUN apk add --no-cache gcc musl-dev
WORKDIR /build
COPY backend/go.mod backend/go.sum ./
RUN go mod download
COPY backend/ ./
RUN CGO_ENABLED=1 go build -o last-deploy ./cmd/last-deploy

# Build frontend
FROM node:20-alpine AS frontend-builder
WORKDIR /build
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

# Runtime
FROM alpine:3.21
RUN apk add --no-cache ca-certificates docker-cli docker-cli-compose
WORKDIR /app
COPY --from=backend-builder /build/last-deploy .
COPY --from=frontend-builder /build/dist ./static
ENV LAST_DEPLOY_STATIC_DIR=/app/static
EXPOSE 8080
CMD ["./last-deploy"]
