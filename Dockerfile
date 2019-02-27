ARG GO_VERSION=latest

FROM golang:${GO_VERSION} AS build

ARG GOMETALINTER_VERSION=3.0.0

RUN url="https://github.com/alecthomas/gometalinter/releases/download/v${GOMETALINTER_VERSION}/gometalinter-${GOMETALINTER_VERSION}-linux-amd64.tar.gz" \
 && curl -sSL "$url" | tar -xzC /usr/local/bin --strip-components 1

WORKDIR /code

COPY go.mod go.sum ./

RUN go mod download

COPY . .

RUN make build


FROM alpine AS release

COPY --from=build /code/aws-env /usr/local/bin

ENTRYPOINT ["aws-env"]
