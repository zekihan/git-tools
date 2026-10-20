ARG GO_IMAGE=golang:1.27.3-alpine3.24@sha256:f2c872bcee949b7be9c490efd02b48a8a2fbf947cdf533f1c58525ec9d8170fd
ARG RUNTIME_IMAGE=alpine:3.24@sha256:294b683cb724975bec92580e1e685676bd4b50bda910ddb8c51d4cabeaec77e6
FROM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=1.0.0
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY *.go ./
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -trimpath -ldflags="-s -w -X main.version=${VERSION#v}" -o /out/git-tools .

FROM ${RUNTIME_IMAGE}
RUN apk add --no-cache ca-certificates git openssh-client
COPY --from=build /out/git-tools /usr/local/bin/git-tools
WORKDIR /work
ENTRYPOINT ["git-tools"]
CMD ["help"]
