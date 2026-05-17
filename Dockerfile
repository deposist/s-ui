FROM --platform=$BUILDPLATFORM node:alpine AS front-builder
WORKDIR /app
COPY frontend/ ./
RUN npm install && npm run build

FROM golang:1.25-alpine AS backend-builder
WORKDIR /app
ARG TARGETARCH
ARG TARGETVARIANT
ARG CRONET_GO_VERSION=e4926ba205fae5351e3d3eeafff7e7029654424a
ARG CRONET_GO_DOWNLOAD_DATE=2026-05-17
ENV CGO_ENABLED=1
ENV CGO_CFLAGS="-D_LARGEFILE64_SOURCE"
ENV GOARCH=$TARGETARCH

RUN apk update && apk add --no-cache \
    gcc \
    musl-dev \
    libc-dev \
    make \
    git \
    wget \
    unzip \
    bash \
    curl

ENV CC=gcc

# SagerNet/cronet-go does not publish prebuilt release assets keyed by commit
# SHA. Keep the source pin synchronized with release.yml and download the
# latest prebuilt asset as of CRONET_GO_DOWNLOAD_DATE until pinned assets exist.
RUN CRONET_ARCH="$TARGETARCH" && \
    CRONET_URL="https://github.com/SagerNet/cronet-go/releases/latest/download/libcronet-linux-${CRONET_ARCH}.so"; \
    echo "cronet-go source pin: ${CRONET_GO_VERSION}; prebuilt asset fallback date: ${CRONET_GO_DOWNLOAD_DATE}" && \
    echo "Downloading $CRONET_URL" && \
    wget -q -O ./libcronet.so "$CRONET_URL" && \
    chmod 755 ./libcronet.so

COPY . .
COPY --from=front-builder /app/dist/ /app/web/html/

RUN if [ "$TARGETARCH" = "arm" ]; then export GOARM=7; [ "$TARGETVARIANT" = "v6" ] && export GOARM=6; fi; \
    go build -ldflags="-w -s" \
    -tags "with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,with_tailscale" \
    -o sui main.go

FROM alpine
# Match defaultValueMap["timeLocation"] in service settings.
ENV TZ=Europe/Moscow
WORKDIR /app
RUN set -ex && apk add --no-cache --upgrade bash tzdata ca-certificates nftables
COPY --from=backend-builder /app/sui /app/libcronet.so /app/
COPY entrypoint.sh /app/
ENTRYPOINT [ "./entrypoint.sh" ]
