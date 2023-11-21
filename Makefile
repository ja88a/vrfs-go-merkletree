.PHONY: protos run

setup:
	go work init
	go work use ./libs/rpcapi
	go work use ./libs/merkletree
	go work use ./libs/config
	go work use ./libs/logger
	go work use ./libs/redis
	go work use ./fileserver
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
	go run server/main.go

run-fileserver:
	go run fileserver/main.go

docker-build-vrfs:
	docker build -f ./server/Dockerfile -t vrfs-api:latest .

docker-build-fserver:
	docker build -f ./fileserver/Dockerfile -t vrfs-fs:latest .

docker-compose-up:
	docker compose up --force-recreate --remove-orphans

docker-cleanup:
	docker container rm -v vrfs-fs vrfs-api vrfs-cache && docker image rm -f  vrfs-fs vrfs-api

demo-run-upload:
	go run ./client -action upload -updir ./fs-playground/forupload/catyclops

demo-run-download:
	go run ./client -action download \
		-downdir ./fs-playground/downloaded \
		-fileset fs-c32c7fac4685a43c2e7806d13bdd737a12f56322a65d23e29d81ae0a34a13019 \
		-index 5
