module github.com/ja88a/vrfs-go-merkletree/server

go 1.21.4

replace github.com/ja88a/vrfs-go-merkletree/libs/config v0.0.0 => ../libs/config

replace github.com/ja88a/vrfs-go-merkletree/libs/logger v0.0.0 => ../libs/logger

replace github.com/ja88a/vrfs-go-merkletree/libs/rpcapi v0.0.0 => ../libs/rpcapi

replace github.com/ja88a/vrfs-go-merkletree/libs/redis v0.0.0 => ../libs/redis

replace github.com/ja88a/vrfs-go-merkletree/libs/merkletree v0.0.0 => ../libs/merkletree

require github.com/ja88a/vrfs-go-merkletree/libs/rpcapi v0.0.0

require github.com/ja88a/vrfs-go-merkletree/libs/config v0.0.0

require github.com/ja88a/vrfs-go-merkletree/libs/logger v0.0.0

require (
	github.com/ja88a/vrfs-go-merkletree/libs/merkletree v0.0.0
	github.com/ja88a/vrfs-go-merkletree/libs/redis v0.0.0
	google.golang.org/grpc v1.59.0
)

require google.golang.org/protobuf v1.31.0 // indirect

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/ilyakaznacheev/cleanenv v1.5.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/redis/go-redis/v9 v9.3.0 // indirect
	github.com/rs/zerolog v1.31.0 // indirect
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/sys v0.12.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)
