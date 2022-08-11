all: linux windows darwin checksum

linux:
	GOOS=linux go build -o bin/tos3-server_linux ./cmd/tos3-server

windows:
	GOOS=windows go build -o bin/tos3-server_windoows ./cmd/tos3-server

darwin:
	GOOS=darwin go build -o bin/tos3-server_darwin ./cmd/tos3-server

checksum:
	shasum -a 256 ./bin/*
