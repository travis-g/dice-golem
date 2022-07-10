FROM golang:alpine as builder
RUN mkdir /build
ADD . /build/
WORKDIR /build
RUN apk --update add --no-cache ca-certificates git \
    && go get -v -t -d ./... \
    && CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -ldflags '-extldflags "-static"' -o dice-golem .
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=builder /build/dice-golem /opt/dice-golem
WORKDIR /opt/dice-golem
CMD ["./dice-golem"]
