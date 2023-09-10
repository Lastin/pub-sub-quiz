generate_grpc:
	protoc --go_out=. --go_opt=paths=source_relative \
        --go-grpc_out=. --go-grpc_opt=paths=source_relative \
        proto/*.proto


build_linuxAmd64:
	GOOS=linux GOARCH=amd64 go build -o bin/server cmd/server/*.go
	GOOS=linux GOARCH=amd64 go build -o bin/client cmd/client/*.go

build_darwinAmd64:
	GOOS=darwin GOARCH=amd64 go build -o bin/server cmd/server/*.go
	GOOS=darwin GOARCH=amd64 go build -o bin/client cmd/client/*.go

build_windowsAmd64:
	GOOS=windows GOARCH=amd64 go build -o bin/server.exe cmd/server/*.go
	GOOS=windows GOARCH=amd64 go build -o bin/client.exe cmd/client/*.go