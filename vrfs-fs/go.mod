module github.com/ja88a/vrfs-go-merkletree/vrfs-fs

go 1.25.0

replace github.com/ja88a/vrfs-go-merkletree/libs/config v0.0.0 => ../libs/config

replace github.com/ja88a/vrfs-go-merkletree/libs/logger v0.0.0 => ../libs/logger

replace github.com/ja88a/vrfs-go-merkletree/libs/rpcapi v0.0.0 => ../libs/rpcapi

replace github.com/ja88a/vrfs-go-merkletree/libs/merkletree v0.0.0 => ../libs/merkletree

require (
	github.com/ja88a/vrfs-go-merkletree/libs/config v0.0.0
	github.com/ja88a/vrfs-go-merkletree/libs/logger v0.0.0
	github.com/ja88a/vrfs-go-merkletree/libs/merkletree v0.0.0
	github.com/ja88a/vrfs-go-merkletree/libs/rpcapi v0.0.0
	google.golang.org/grpc v1.83.1
)

require (
	github.com/BurntSushi/toml v1.2.1 // indirect
	github.com/ilyakaznacheev/cleanenv v1.5.0 // indirect
	github.com/joho/godotenv v1.5.1 // indirect
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.19 // indirect
	github.com/rs/zerolog v1.31.0 // indirect
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
	olympos.io/encoding/edn v0.0.0-20201019073823-d3554ca0b0a3 // indirect
)
