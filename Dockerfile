ARG GO_IMAGE=golang:1.27.3-alpine3.24@sha256:dbaa054703263d06c8b49576667418473b6231f8c0fa69dee103207fdb8e3654
ARG RUNTIME_IMAGE=alpine:3.24@sha256:3f4d6b32e26703c6082839234f43769d5046b3ac7c910033911d1e71d8cd630a
FROM ${GO_IMAGE} AS build
ARG TARGETOS
ARG TARGETARCH
ARG VERSION=2.0.0
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
