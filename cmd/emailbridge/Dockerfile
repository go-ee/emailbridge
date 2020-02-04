FROM alpine:latest

RUN mkdir /app
WORKDIR /app
COPY emailbridge .

ENTRYPOINT ["/app/emailbridge"]