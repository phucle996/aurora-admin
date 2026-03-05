# syntax=docker/dockerfile:1.7

FROM node:24-alpine AS frontend-builder
WORKDIR /frontend

COPY package*.json ./
RUN npm ci --no-fund --no-audit

COPY . .
RUN npm run build

FROM golang:1.26-alpine AS backend-builder
WORKDIR /app

RUN apk add --no-cache git build-base

COPY go.mod go.sum ./
RUN go mod download

COPY . .
COPY --from=frontend-builder /frontend/internal/app/dist ./internal/app/dist

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /app/bin/admin-server ./cmd/server

FROM alpine:3.21 AS runner
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata \
  && addgroup -S nonroot \
  && adduser -S -G nonroot -h /app -s /sbin/nologin nonroot

COPY --from=backend-builder --chown=nonroot:nonroot /app/bin/admin-server /app/server
COPY --from=backend-builder --chown=nonroot:nonroot /app/dist /app/dist

USER nonroot:nonroot

EXPOSE 3000
ENTRYPOINT ["/app/server"]
