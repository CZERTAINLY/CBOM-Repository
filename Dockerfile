########################
# Build Stage
########################
FROM golang:1.25-alpine3.22 AS builder

# We need to install git to be able to identify version from git tags
RUN apk add --no-cache git

ARG VERSION=dev
ENV CGO_ENABLED=0 \
    GOFLAGS="-trimpath" \
    LDFLAGS="-s -w -X main.version=${VERSION}"

WORKDIR /src

# Better layer caching for deps
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download

# Copy the rest and build
COPY . .
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    go build -ldflags "${LDFLAGS}" -o /out/cbom-repository ./cmd/cbom-repository

########################
# Run Stage
########################
FROM alpine:3.23

LABEL org.opencontainers.image.authors="CZERTAINLY <support@czertainly.com>"

# add non root user czertainly
RUN addgroup --system --gid 10001 czertainly && adduser --system --home /opt/czertainly --uid 10001 --ingroup czertainly czertainly

COPY --from=builder /out/cbom-repository /usr/local/bin/cbom-repository

ENV APP_LOG_LEVEL=INFO

EXPOSE 8080

USER 10001

ENTRYPOINT ["/usr/local/bin/cbom-repository"]
