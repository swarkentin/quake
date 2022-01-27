# Builds a REST Proxy for the gRPC server
#
#  See also
#    Mayhem for API: https://mayhem4api.forallsecure.com/
#    grpc-gateyway: https://github.com/grpc-ecosystem/grpc-gateway
#
#  Running (the gRPC server is NOT port forwarded, only the proxy)
#   docker run -it --rm -p 8081:8081 --name quake-rest-proxy quake-rest-proxy
#
#  Getting the swagger spec:
#   docker cp quake-rest-proxy:/opt/quake/api/v1/quake_api.swagger.json .
#
#  Running with mapi
#    mapi run quake-grpc 5sec ./quake_api.swagger.json --url http://localhost:8081 --interactive
#
#  Calling the proxy API
#    curl --location --request POST "http://localhost:8081/quake.api.v1.QuakeService/ListEarthquakes" \
#         --header 'Content-Type: text/plain' \
#         --data-raw '{
#           "magnitude": "MAGNITUDE_ALL",
#           "past": "PAST_30DAYS",
#           "limit": 5,
#           "details": false
#         }'
FROM golang:1.17.6

# Install protoc, protocol buffer compiler
RUN apt update && \
    apt install -y protobuf-compiler \
    # supervisor allows launching the gRPC server AND rest proxy
                   supervisor && \
    rm -rf /var/lib/apt/lists/*

WORKDIR /opt/quake

COPY . .

RUN go install \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway \
    github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2 \
    google.golang.org/protobuf/cmd/protoc-gen-go \
    google.golang.org/grpc/cmd/protoc-gen-go-grpc

WORKDIR /opt/quake/api/v1

# Generate code from .proto
RUN make

# Generate proxy stubs from .proto files
RUN make proxy_stub

# Build the gRPC server
WORKDIR /opt/quake/cmd/quake-server
RUN go build

# Build the rest proxy
WORKDIR /opt/quake/mayhem-for-api
RUN go build -o rest-proxy

# Generate the swagger specification
WORKDIR /opt/quake/api/v1
RUN protoc -I . --openapiv2_out . \
    --openapiv2_opt logtostderr=true \
    --openapiv2_opt generate_unbound_methods=true \
    quake_api.proto

# 50051 - gRPC server
# 8081  - rest proxy
EXPOSE 50051 8081

# Configure supervisor so that the server and proxy are run
# with supervisor
ADD mayhem-for-api/supervisord.conf /etc/supervisor/conf.d/supervisord.conf
ENTRYPOINT ["/usr/bin/supervisord"]