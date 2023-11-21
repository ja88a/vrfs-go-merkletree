###################
# BASE FOR VERSIONING
###################

FROM golang:1.21-alpine3.18 As base

# Root working dir
WORKDIR /app-src

COPY ./fileserver/.semver.yaml ./.semver.yaml

# Target directory for built artefacts
RUN mkdir ./dist

# Manage the service's Versioning
RUN go install github.com/maykonlf/semver-cli/cmd/semver@v1.0.2
RUN semver get release > ./dist/version


###################
# BUILD FOR LOCAL DEVELOPMENT
###################

FROM golang:1.21-alpine3.18 As build

# Root working dir
WORKDIR /app-src

# Go Caching
#ENV CGO_ENABLED=1
RUN go env -w GOCACHE=/go-cache
RUN go env -w GOMODCACHE=/gomod-cache

# Copy Go modules and dependencies to image
COPY ./libs ./libs
COPY ./fileserver ./fileserver
COPY ./config ./config

# Switch to the VRFS module dir
WORKDIR /app-src/fileserver

# Download Go modules and dependencies
RUN --mount=type=cache,target=/gomod-cache \
    go mod download

# Report the generated version
COPY --chown=gorker:gorker --from=base /app-src/dist ./dist

# Run the build command which creates the production bundle
RUN --mount=type=cache,target=/go-cache --mount=type=cache,target=/gomod-cache \
    go build -o dist/vrfs-fs ./main.go


###################
# PRODUCTION
###################

FROM  gcr.io/distroless/base-debian12 As production

WORKDIR /app

# Copy the bundled code from the build stage to the production image
COPY --chown=gorker:gorker --from=build /app-src/fileserver/dist ./
COPY --chown=gorker:gorker --from=build /app-src/config/server-fs.yml ./config/server-fs.yml

# Ensure the process will run under the user 'gorker'
#USER gorker

# Optional specification of the port to expose
EXPOSE 9000

ENTRYPOINT ["./vrfs-fs"]
