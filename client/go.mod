module github.com/ja88a/vrfs-go-merkletree/client

go 1.25.0

replace github.com/ja88a/vrfs-go-merkletree/libs/merkletree v0.0.0 => ../libs/merkletree

replace github.com/ja88a/vrfs-go-merkletree/libs/rpcapi v0.0.0 => ../libs/rpcapi

require (
	github.com/ja88a/vrfs-go-merkletree/libs/merkletree v0.0.0
	github.com/ja88a/vrfs-go-merkletree/libs/rpcapi v0.0.0
	google.golang.org/grpc v1.83.1
)

require (
	golang.org/x/net v0.55.0 // indirect
	golang.org/x/sys v0.45.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260526163538-3dc84a4a5aaa // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
