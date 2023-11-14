###################
# BUILD FOR LOCAL DEVELOPMENT
###################

FROM golang:1.21-alpine3.18 As build

# Create app directory
WORKDIR /usr/src/app

# Bundle app source
COPY --chown=node:node . .

# copy Go modules and dependencies to image
COPY go.mod ./

# download Go modules and dependencies
RUN go mod download

# Manage the service's Versioning
RUN go install github.com/maykonlf/semver-cli/cmd/semver@v1.0.2
COPY .semver.yaml .
RUN semver get release > version

# Run the build command which creates the production bundle
RUN go build -o vrfs-server main.go

###################
# PRODUCTION
###################

FROM golang:1.21-alpine3.18 As production

WORKDIR /usr/src/app

# Copy the bundled code from the build stage to the production image
COPY --chown=node:node --from=build /usr/src/app/version ./version
COPY --chown=node:node --from=build /usr/src/app/dist ./dist

# Ensure the Node process will run under the user 'node'
USER node

# Optional specification of the port to expose
EXPOSE 3000

ENTRYPOINT ["./vrfs-server"]
