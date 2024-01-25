.PHONY: protos run

setup-workspace:
	go work init
	go work use ./libs/rpcapi
	go work use ./libs/merkletree
	go work use ./libs/config
	go work use ./libs/logger
	go work use ./libs/db
	go work use ./vrfs-fs
	go work use ./vrfs-api
	go work use ./client

versions:
	@go version
	@protoc --version

protos:
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		libs/rpcapi/protos/v1/vrfs-api/vrfs.proto
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		libs/rpcapi/protos/v1/vrfs-fs/fileserver.proto

lint:
	golangci-lint run libs/config/... libs/db/... libs/logger/... \
		libs/merkletree/... libs/rpcapi/... \
		vrfs-fs/... vrfs-api/... client/...

run-vrfs:
	go run vrfs-api/main.go

run-fs:
	go run vrfs-fs/main.go

docker-build-vrfs:
	docker build -f ./vrfs-api/Dockerfile -t vrfs-api:latest .

docker-build-fserver:
	docker build -f ./vrfs-fs/Dockerfile -t vrfs-fs:latest .

docker-compose-up:
	docker compose up --build --force-recreate --remove-orphans

docker-cleanup:
	docker container rm -v vrfs-fs vrfs-api vrfs-cache && docker image rm -f  vrfs-fs vrfs-api

fs-playground-cleanup:
	rm -rf ./fs-playground/forupload/*/
	rm -rf ./fs-playground/downloaded
	sudo rm -rf ./fs-playground/fs_client_files

demo-run-upload:
	tar -xzf ./fs-playground/forupload/catyclops.tar.gz -C ./fs-playground/forupload/
	go run ./client -action upload \
		-updir ./fs-playground/forupload/catyclops

demo-run-download:
	go run ./client -action download \
		-downdir ./fs-playground/downloaded \
		-fileset fs-10f652e11f5e2f799481ed02d45a74bcf3d62dea3200ad08120bba43c242f5fb \
		-index 5
