.PHONY: protos run

setup:
	go work init
	go work use ./libs/protos
	go work use ./libs/merkletree
	go work use ./server
	go work use ./client

versions:
	@go version
	@protoc --version

protos:
	protoc --go_out=. --go_opt=paths=source_relative \
		 --go-grpc_out=. --go-grpc_opt=paths=source_relative \
		libs/protos/v1/vrfs/vrfs.proto

run:
	go run server/main.go &
	go run client/main.go
