# Verifiable Remote Files Storage

## Purpose

### General

The Verifiable Remote File Storage service aims at checking the consistency of uploaded then downloaded files, based on file hashes and their MerkleTree proofs.

This is the mono repository for the Go-based implementation of 2 backend services and 1 CLI client.

The gRPC protocol is used for optimal client-server and server-server communications.


### Protocol Overview

The overal implemented protocol for uploading or downloading local files to the remote file storage service and always have the files verified based on the generation of a Merkle Tree root and proofs for the checking the leaf values, the file hashes here:

![Implemented Protocol Overview](./doc/assets/VRFS_overview-protocol_v1.png)

* The VRFS Service handles the creds/access to the [external] NFS service
* Files are directly uploaded to & downloaded from the NFS service
* VRFS retrieves the file hashes from the NFS server, for building its Merkle Tree

This protocol results in: 
* 6 steps to remotely store the files - 1 client command: 1 API & n file uploads requests + 1 VRFS-FS API request
* 3 steps to retrieve & verify a file - 1 client command: 1 API & 1 file download requests

3 main components have been implemented:
1. The VRFS API Service - The core component of this protocol, exposing a gRPC API
2. A basic File Storage service exposing a gRPC API to batch upload files and download them individually, or get the list of stored files' hashes
3. A CLI client to execute the 2 main upload and download operations, along with MerkleProof-based file hash verifications


## Instructions

### Running the Servers

A Docker Compose setup will enhance the below manual mode for running the 2 server modules:

```shell
# Run locally the File Storage server
$ go run ./fileserver

# Run locally the VRFS server
$ go run ./server
```

### Running the CLI client

List the available client CLI parameters:

```shell
$ go run ./client -h
```

File Upload & Verify protocol: Upload all files of a local directory to the remote file storage server:

```shell
# With default service endpoints
$ go run ./client -action upload -updir ./fs-playground/forupload/catyclops

# Or by specifying the service endpoints and a max chunk size
$ go run ./client -action upload -updir ./fs-playground/forupload/catyclops \
    -api vrfs-server:50051 \
    -fs nfs-server:9000 \
    -chunk 1024
```

Download locally a file from VFRS API & the File Storage services and have it verified:

```shell
# 
$ go run ./client -action download \
    -fileset fs-10B..7E21 \
    -index 5
    -downdir ./fs-playground/downloaded
```

### Building Executable Go Modules

```
go build ./client -o ./dist/vrfs-client
go build ./server -o ./
```

### Modules Runtime Config

The CLI client is configurable via command parameters it exposes.

The VRFS & FS server configurations rely on their dedicated yml config file in [config](./config), those
parameters can be overridden via optional `.env` files or via runtime environment variables. Refer to 
the cleanenv solution and its integration made in the lib utils [config](./libs/utils/config/config.go).


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

Overview of the VRFS Service and its main components:

![VRFS Service Overview][./doc/assets/VRFS_overview-service_v1.png]

Overview of the considered overall, scalable solution to be implemented: 

![VRFS Solution Overview][./doc/assets/VRFS_overview-solution_v1b.png]


## Development status

The depicted files' verification protocol on client side has not yet been finalized.
The remaining key challenge is about the efficient serialization of the MerkleTree Proofs to be communicated
to the clients on every file download verification, as well as for their DB storage.

A persistence layer for the VRFS Service is required to be implemented, via a DB ORM integration
and a moreover distributed/embedded memory cache solution, such as Redis-like solutions.

The computation models and their settings for the backbone Merkle Tree reference is to be 
further refined and benchmarked, per the integration use case(s) and corresponding optimization 
requirements for ad-hoc computation, storage and transport .


## Solution Readiness - Status

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

Actual services' JSON logs should be reported to a remote log watcher to review them, monitor the service and/or automate runtime alerts triggering.

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

The services actually handle a version number through their configuration file.

#### Automated Tests

Automated unit and integration testing have not been addressed yet.

E2E tests & a continuous integration / deployment flows should be implemented.

#### CLI Client

A Cordoba-like integration should be considered if the CLI client is given a priority.

A logger might also have to be integrated.
