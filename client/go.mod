module github.com/ja88a/vrfs-go-merkletree/client

go 1.21.4

replace github.com/ja88a/vrfs-go-merkletree/libs/merkletree v0.0.0 => ../libs/merkletree

replace github.com/ja88a/vrfs-go-merkletree/libs/protos v0.0.0 => ../libs/protos

replace github.com/ja88a/vrfs-go-merkletree/libs/utils v0.0.0 => ../libs/utils

require (
	github.com/ja88a/vrfs-go-merkletree/libs/merkletree v0.0.0
	github.com/ja88a/vrfs-go-merkletree/libs/protos v0.0.0
	google.golang.org/grpc v1.59.0
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.5 // indirect
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/spf13/cobra v1.8.0
	golang.org/x/net v0.14.0 // indirect
	golang.org/x/sys v0.11.0 // indirect
	golang.org/x/text v0.12.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/protobuf v1.31.0 // indirect
)
