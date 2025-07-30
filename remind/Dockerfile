# syntax=docker/dockerfile:1
ARG GO_VERSION=1.23.1
ARG TARGET_ARCH=arm64
ARG TARGET_OS=linux

FROM golang:${GO_VERSION}-bookworm AS base
WORKDIR /src
ARG TARGET_OS
ARG TARGET_ARCH
ENV CGO_ENABLED=0 \
    GOOS=${TARGET_OS} \
    GOARCH=${TARGET_ARCH}
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=bind,source=go.mod,target=go.mod \
    --mount=type=bind,source=go.sum,target=go.sum \
    go mod download -x

FROM --platform=${BUILDPLATFORM} base AS build
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    go build -tags lambda.norpc -o /usr/local/bin/app

FROM --platform=${BUILDPLATFORM} base AS vet
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    go vet

FROM --platform=${BUILDPLATFORM} base AS test
RUN --mount=type=cache,target=/go/pkg/mod/ \
    --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=bind,target=. \
    go test

FROM public.ecr.aws/lambda/provided:al2023 AS local
COPY --from=build /usr/local/bin/app /usr/local/bin/app
ENTRYPOINT ["/usr/local/bin/aws-lambda-rie"]
CMD ["app"]

# TODO: 実行時エラー "Runtime.InvalidEntrypoint" の原因を調査する
# FROM gcr.io/distroless/static-debian12:nonroot-${TARGET_ARCH} AS final
FROM public.ecr.aws/lambda/provided:al2023 AS final
ARG GIT_COMMIT_HASH
ARG GIT_REPO_URL
ARG BUILD_DATE
LABEL org.opencontainers.image.title="remind" \
      org.opencontainers.image.description="AWS Lambda 上での実行を想定した、イベント情報をリマインドするアプリ" \
      org.opencontainers.image.revision="${GIT_COMMIT_HASH}" \
      org.opencontainers.image.source="${GIT_REPO_URL}" \
      org.opencontainers.image.created="${BUILD_DATE}"
COPY --from=build /usr/local/bin/app /usr/local/bin/app
ENTRYPOINT ["app"]
