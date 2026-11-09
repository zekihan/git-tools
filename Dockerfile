ARG GO_IMAGE=golang:1.27.3-alpine3.24@sha256:fa3d596898d9f126af7c01d14ee7597bab3f39cccf392ef36b69fe77cd681f18
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
RUN apk add --no-cache ca-certificates git openssh-client
COPY --from=build /out/git-tools /usr/local/bin/git-tools
WORKDIR /work
ENTRYPOINT ["git-tools"]
CMD ["help"]
