FROM golang:1.26-alpine AS build-stage

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o /heromail ./cmd/heromail

FROM alpine:3.22

RUN addgroup -S heromail && adduser -S -G heromail heromail
COPY --from=build-stage /heromail /usr/local/bin/heromail
USER heromail
EXPOSE 8080
ENTRYPOINT ["/usr/local/bin/heromail"]
