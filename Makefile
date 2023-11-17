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
		libs/rpcapi/protos/v1/vrfs/vrfs.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		libs/rpcapi/protos/v1/fileserver/fileserver.proto

run-vrfs:
	go run server/main.go &
	go run client/main.go

run-filetranfer:
	go run fileserver/main.go &
	go run client/main.go

docker-build-vrfs:
	docker build -f ./server/Dockerfile -t vrfs-api:latest .

docker-build-fserver:
	docker build -f ./fileserver/Dockerfile -t vrfs-fs:latest .

docker-compose-up:
	docker compose up --force-recreate --remove-orphans

docker-cleanup:
	docker container rm -v vrfs-fs vrfs-api vrfs-cache && docker image rm -f  vrfs-fs vrfs-api vrfs-cache