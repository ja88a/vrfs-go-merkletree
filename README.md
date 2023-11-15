# Verifiable Remote Files Storage

## Purpose

### General

The Verifiable Remote File Storage service aims at checking the consistency of uploaded then downloaded files, based on file hashes and their MerkleTree proofs.

This is the mono repository for the Go-based implementation of 2 backend services and 1 CLI client.

The gRPC protocol is used for optimal client-server and server-server communications.


### Protocol


## Instructions

### Running client commands

List the available client CLI parameters:

```shell
$ go run ./client -h
```

Upload all files of a local directory to the remote file storage server:

```shell
# With default service endpoints
$ go run ./client -action upload -dir ./fs-playground/forupload/catyclops

# Or by specifying the service endpoints
$ go run ./client -action upload -dir ./fs-playground/forupload/catyclops \
    -api vrfs-server:50051 \
    -rfs nfs-server:9000
```

## Development Framework

### Tools

Pre-requisites:
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

A Go workspace is used for handling the different modules part of this monorepo.

Refer to the workspace config file: [`go.work`](./go.work).

Adding a new module to the workspace:
```
$ mkdir moduleX
$moduleX/ go mod init github.com/ja88a/vrfs-go-merkletree/moduleX
$ go work use ./moduleX
```


## Architecture



## Production Readiness - Status

### Missing for Production

#### APIs Traffic management

No user authentication mechanism has been implemented to protect the access to the service APIs.

Available client authentication options:
* API Keys support
* ECDSA signature - a digital signature made from an externally owned account

External services acting as load balancers and an API Gateway should be integrated in order to deal with:
* Load balancing
* API requests routing & APIs versioning
* Communications encryption
* User authentication & permissions

#### Logs Reporting

The services' JSON logs should be reported to a remote log watcher to review them, monitor the service and/or automate runtime alerts triggering.

Example of available 3rd party solutions: Sentry.io - Refer to [sentry-go](https://github.com/getsentry/sentry-go).

#### Services Monitoring

A monitoring infra is to be added and servers should report their usage & performance statistics.

The integration of a Prometheus-like time serie events database should be considered so that each servers report their stats.

Complementary tools like a monitoring dashboard, e.g. Grafana, and runtime alerts management, e.g. Kibana, would be welcome.

### Required Improvements

#### End-to-end TLS Encryption

Actual gRPC communications should rely on TLS encryption over HTTP.

X.509 certificates are to be deployed at servers' level and secure connections initiated by the client. 

Refer to actual `grpc.WithTransportCredentials`.

#### API Versions Management

A basic versioning mechanism is actually implemented for managing the Protobuf-based gRPC stubs integrated by the client and the servers.

The Docker images are tagged using the semver CLI tool.
