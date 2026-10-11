.PHONY: test build dist

test:
	go test ./...

build:
	go build -trimpath -o bin/syncmkv ./cmd/syncmkv

dist:
	GOOS=linux GOARCH=amd64 go build -trimpath -o bin/syncmkv-linux-amd64 ./cmd/syncmkv
	GOOS=windows GOARCH=amd64 go build -trimpath -o bin/syncmkv-windows-amd64.exe ./cmd/syncmkv
