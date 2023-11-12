# Verifiable Remote Files Storage

## Purpose

### General

The Verifiable Remote File Storage service aims at checking the consistency of uploaded then downloaded files, based on files' secp256k hash and their MerkleTree proofs.

This is the mono repository for the Go-based implementation of the Backend services and a Client.

The gRPC protocol is used for optimal client-server interactions.

### Protocol


## Dev Framework Installation

### Tools

Requirements:
* [Go](https://golang.org/) dev framework >= v1.18 - [install](https://go.dev/doc/install)
* [Protocol buffer](https://developers.google.com/protocol-buffers) compiler >= v3 - [install](https://grpc.io/docs/protoc-installation/) 
* Protobuf gRPC Go plugins to generate the client SDK and server API stubs - [install](https://grpc.io/docs/languages/go/quickstart/#prerequisites)
* [Docker](https://www.docker.com/) for running the backend services in virtual containers - [install](https://docs.docker.com/engine/install/)
* `make` for benefiting from the available [`Makefile`](./Makefile) commands

Versions used while developing:
* Go : `go1.21.4 linux/amd64`
* protoc : `libprotoc 3.12.4`
* Docker : `24.0.7, build afdd53b`

### Go Workspace

A Go workspace is used for handling this monorepo.

Refer to the workspace config file in [`go.work`](./go.work).

Adding a new module to the workspace:
```
$ mkdir moduleX
$moduleX/ go mod init github.com/ja88a/vrfs-go-merkletree/moduleX
$ go work use ./moduleX
$moduleX/ go mod tidy
```


## Architecture



## Production Readiness - Status

### Missing for Production

#### API access management & Authentication

No authentication mechanism has been implemented to protect the access to the server API.

Available client authentication options:
* API Keys support
* ECDSA signature - a digital signature made from an externally owned account

#### Logs Reporting

The service logs should be reported to a remote log watcher to review them, monitor the service and/or automate runtime alerts triggering.

Example of available 3rd party solutions: Sentry.io - Refer to [sentry-go](https://github.com/getsentry/sentry-go).

### Required Improvements

#### API Versions Management

A basic versioning mechanism is implemented for managing the Protobuf-based gRPC stubs: based on the proto files' naming to include a version number.

The Docker images are tagged using the semver CLI tool.
