# --- build stage -------------------------------------------------------
FROM golang:1.25-alpine AS builder

WORKDIR /src

# Cache module downloads separately from source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY main.go ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/key-value-cache .

# --- runtime stage -------------------------------------------------------
FROM alpine:3.20

RUN apk add --no-cache ca-certificates wget \
	&& addgroup -S kvcache && adduser -S kvcache -G kvcache

COPY --from=builder /out/key-value-cache /usr/local/bin/key-value-cache

USER kvcache
EXPOSE 7171

# GOMAXPROCS is auto-tuned from the container's CPU quota at startup
# (go.uber.org/automaxprocs); GOMEMLIMIT should be set to ~90% of the
# container's memory limit so the GC stays ahead of an OOM kill.
ENV PORT=7171

HEALTHCHECK --interval=10s --timeout=2s --start-period=5s --retries=3 \
	CMD wget -qO- http://127.0.0.1:${PORT}/health || exit 1

ENTRYPOINT ["/usr/local/bin/key-value-cache"]
