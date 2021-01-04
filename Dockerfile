FROM golang:alpine as builder

RUN apk add build-base

COPY . /src
WORKDIR /src/cmd/helpbot
ENV CGO_ENABLED 1
RUN go mod download
RUN go install -ldflags '-s -w -extldflags "-static"' .

FROM scratch

COPY --from=builder /go/bin/helpbot /go/bin/helpbot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT [ "/go/bin/helpbot" ]