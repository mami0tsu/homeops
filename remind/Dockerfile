# syntax=docker/dockerfile:1
ARG GO_VERSION=1.23.1
ARG TARGET_ARCH=arm64
ARG TARGET_OS=linux

FROM golang:${GO_VERSION}-bookworm AS base
WORKDIR /src
ENV CGO_ENABLED=0 \
    GOOS=${TARGET_OS} \
    GOARCH=${TARGET_ARCH}
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download -x

FROM --platform=$BUILDPLATFORM base AS build
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    go build -tags lambda.norpc -o /usr/local/bin/app

FROM --platform=$BUILDPLATFORM base AS vet
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    go vet

FROM --platform=$BUILDPLATFORM base AS test
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    go test

FROM public.ecr.aws/lambda/provided:al2023 AS local
COPY --from=build /usr/local/bin/app /usr/local/bin/remind
ENTRYPOINT ["/usr/local/bin/aws-lambda-rie"]
CMD ["remind"]

# TODO: 実行時エラー "Runtime.InvalidEntrypoint" の原因を調査する
# FROM gcr.io/distroless/static-debian12:nonroot-${TARGET_ARCH} AS final
FROM public.ecr.aws/lambda/provided:al2023 AS final
COPY --from=build /usr/local/bin/app /usr/local/bin/remind
ENTRYPOINT ["remind"]
