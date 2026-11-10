ARG GO_IMAGE=golang:1.27.4-alpine3.24@sha256:0a5f7fe796599ff487605987432e7619ae45beeb2fba4c2ce0b907879932aebf
ARG RUNTIME_IMAGE=alpine:3.24.4@sha256:f59abe656b692d901e184be3445e084786602108599d2a51d229c0f5a48c71bf
FROM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=2.0.0
WORKDIR /src
COPY go.mod go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod go mod download
COPY *.go ./
RUN --mount=type=cache,target=/go/pkg/mod --mount=type=cache,target=/root/.cache/go-build \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w -X main.version=${VERSION#v}" -o /out/git-tools .

FROM ${RUNTIME_IMAGE}
RUN apk add --no-cache ca-certificates git openssh-client tini \
    && addgroup -g 65532 git-tools \
    && adduser -D -u 65532 -G git-tools -h /home/git-tools git-tools \
    && install -d -o 65532 -g 65532 /work
COPY --from=build /out/git-tools /usr/local/bin/git-tools
USER 65532:65532
WORKDIR /work
# Git detaches auto-maintenance after fetches; tini reaps those orphans.
ENTRYPOINT ["/sbin/tini", "--", "git-tools"]
CMD ["help"]
LABEL org.opencontainers.image.source="https://github.com/zekihan/git-tools" \
      org.opencontainers.image.title="git-tools" \
      org.opencontainers.image.description="Git workflow utilities"
