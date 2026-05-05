# syntax=docker/dockerfile:1

# ── build stage ──────────────────────────────────────────────────────────────
FROM golang:1.25-alpine AS builder

WORKDIR /build

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /daimon ./cmd/daimon

# ── runtime stage ─────────────────────────────────────────────────────────────
FROM gcr.io/distroless/static-debian12

COPY --from=builder /daimon /daimon

EXPOSE 3500

ENTRYPOINT ["/daimon", "serve"]
CMD ["--config", "/etc/daimon/config.yaml"]
