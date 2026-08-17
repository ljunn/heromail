FROM golang:1.26-alpine AS build-stage

ARG VERSION=dev
ARG COMMIT=unknown
ARG BUILD_TIME=unknown

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w -X github.com/ljunn/heromail/internal/buildinfo.Version=${VERSION} -X github.com/ljunn/heromail/internal/buildinfo.Commit=${COMMIT} -X github.com/ljunn/heromail/internal/buildinfo.BuildTime=${BUILD_TIME}" -o /heromail ./cmd/heromail

FROM alpine:3.22

RUN apk add --no-cache ca-certificates su-exec && addgroup -S heromail && adduser -S -G heromail heromail
COPY --from=build-stage /heromail /usr/local/bin/heromail
COPY docker/entrypoint.sh /usr/local/bin/heromail-entrypoint
RUN chmod +x /usr/local/bin/heromail-entrypoint
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/heromail-entrypoint"]
CMD ["/usr/local/bin/heromail"]
