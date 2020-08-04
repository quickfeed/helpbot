FROM golang:alpine as builder

COPY . /src
WORKDIR /src
ENV CGO_ENABLED 0
RUN go mod download
RUN go install -ldflags '-s -w' .

FROM scratch

COPY --from=builder /go/bin/helpbot /go/bin/helpbot
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT [ "/go/bin/helpbot" ]
