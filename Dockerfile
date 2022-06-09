ARG GO_VERSION=1.18

# The build stage is used for building the aws-env binary and running tests.
FROM golang:${GO_VERSION} AS build
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
FROM alpine
COPY --from=build /code/aws-env /usr/local/bin
ENTRYPOINT ["aws-env"]
