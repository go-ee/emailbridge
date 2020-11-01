FROM golang:latest as build
ENV CGO_ENABLED=0
ENV GOOS=linux

WORKDIR /go/src/github.com/go-ee/emailbridge
COPY . /go/src/github.com/go-ee/emailbridge

RUN go get ./...

WORKDIR /go/src/github.com/go-ee/emailbridge/cmd/emailbridge/
RUN go build

FROM alpine:latest

COPY --from=build /go/src/github.com/go-ee/emailbridge/cmd/emailbridge/emailbridge /usr/local/bin/emailbridge

CMD ["emailbridge"]