ARG GOLANG_IMAGE=golang:1.25-alpine
ARG ALPINE_IMAGE=alpine:3.21
FROM ${GOLANG_IMAGE} AS build

ARG TARGETOS=linux
ARG TARGETARCH=amd64
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn

WORKDIR /src
ENV GOPROXY=${GOPROXY} \
    GOSUMDB=${GOSUMDB} \
    GOFLAGS=-buildvcs=false
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/pm-api ./cmd/pm-api \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/pm-worker ./cmd/pm-worker \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/pm-migrate-legacy ./cmd/pm-migrate-legacy \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/pm-repair-team-scope ./cmd/pm-repair-team-scope \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/pm-migrate-workflow-artifacts ./cmd/pm-migrate-workflow-artifacts \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/pm-repair-id-sequences ./cmd/pm-repair-id-sequences \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/pm-cover-renderer ./cmd/pm-cover-renderer \
    && CGO_ENABLED=0 GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
      go build -trimpath -ldflags="-s -w" -o /out/pm-cover-worker ./cmd/pm-cover-worker

FROM ${ALPINE_IMAGE}

RUN apk add --no-cache ca-certificates poppler-utils tzdata util-linux-misc \
    && addgroup -S -g 10001 pm \
    && adduser -S -D -H -u 10001 -G pm pm \
    && mkdir -p /data && chown pm:pm /data
COPY --from=build /out/pm-api /usr/local/bin/pm-api
COPY --from=build /out/pm-worker /usr/local/bin/pm-worker
COPY --from=build /out/pm-migrate-legacy /usr/local/bin/pm-migrate-legacy
COPY --from=build /out/pm-repair-team-scope /usr/local/bin/pm-repair-team-scope
COPY --from=build /out/pm-migrate-workflow-artifacts /usr/local/bin/pm-migrate-workflow-artifacts
COPY --from=build /out/pm-repair-id-sequences /usr/local/bin/pm-repair-id-sequences
COPY --from=build /out/pm-cover-renderer /usr/local/bin/pm-cover-renderer
COPY --from=build /out/pm-cover-worker /usr/local/bin/pm-cover-worker

USER 10001:10001
WORKDIR /data
EXPOSE 8081
ENTRYPOINT ["/usr/local/bin/pm-api"]
