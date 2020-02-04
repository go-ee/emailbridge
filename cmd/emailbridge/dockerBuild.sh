go get ./...
go build
docker build -t emailbridge -f Dockerfile .
rm emailbridge
