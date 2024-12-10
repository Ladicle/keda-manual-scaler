FROM --platform=$BUILDPLATFORM golang:1.23 AS builder
ARG TARGETARCH=amd64
ARG TARGETOS=linux

WORKDIR /wrk
COPY . .

ENV GOCACHE=/root/gocache
RUN --mount=type=cache,target=${GOCACHE} \
    --mount=type=cache,target=/go/pkg/mod \
    go mod download
RUN --mount=type=cache,target=${GOCACHE} \
    --mount=type=cache,id=keda-manual-scaler,sharing=locked,target=/go/pkg/mod \
    CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} go build -o scaler

FROM --platform=$BUILDPLATFORM gcr.io/distroless/static:nonroot
COPY --from=builder /wrk/scaler /
ENTRYPOINT ["/scaler"]
