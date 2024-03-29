###################
# BASE FOR VERSIONING
###################

FROM golang:1.21-alpine3.18 As base

# Root working dir
WORKDIR /app-src

COPY ./vrfs-api/.semver.yaml ./.semver.yaml

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
COPY ./vrfs-api ./vrfs-api
COPY ./libs ./libs
COPY ./config ./config

# Switch to the VRFS module dir
WORKDIR /app-src/vrfs-api

# Download Go modules and dependencies
RUN --mount=type=cache,target=/gomod-cache \
    go mod download

# Report the generated version
COPY --chown=gorker:gorker --from=base /app-src/dist ./dist

# Run the build command which creates the production bundle
RUN --mount=type=cache,target=/go-cache --mount=type=cache,target=/gomod-cache \
    go build -o dist/vrfs-server ./main.go


###################
# PRODUCTION
###################

FROM gcr.io/distroless/static-debian12:nonroot As production

WORKDIR /app

# Copy the bundled code from the build stage to the production image
COPY --chown=nonroot:nonroot --from=build /app-src/vrfs-api/dist ./
COPY --chown=nonroot:nonroot --from=build /app-src/config/vrfs-api.yml ./config/vrfs-api.yml

# Ensure the process will run under the user 'gorker'
USER nonroot

# The port to expose
EXPOSE 50051

ENTRYPOINT ["/app/vrfs-server"]
