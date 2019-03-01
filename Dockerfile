ARG GO_VERSION=1.12

# The build stage is used for building the aws-env binary and running tests.
FROM golang:${GO_VERSION} AS build

ARG GO_CI_VERSION=v1.15.0

RUN curl -sfL https://install.goreleaser.com/github.com/golangci/golangci-lint.sh \
    | sh -s -- -b /usr/local/bin ${GO_CI_VERSION}

WORKDIR /code

ENV GO111MODULE=on

COPY go.mod go.sum ./

RUN go mod download

COPY . .

ARG GIT_COMMIT=unknown
ARG BUILD_DATE=unknown
ARG VERSION=0.0.1
ARG BUILD_NUMBER=0

RUN make build

# The release (default) stage is a minimal production image suitable for use
# as the base image to applications.
FROM alpine AS release

COPY --from=build /code/aws-env /usr/local/bin

ENTRYPOINT ["aws-env"]
