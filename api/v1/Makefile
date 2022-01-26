# Makefile to generate client and server stub code for proto-files.

# Dependencies (tools must be available on the path):
# - protoc (Protocol Buffer v3)
# - protoc-gen-go (protoc compiler extension for Go)
#
# That is install Go 1.6 or higher, gRPC, Protocol Buffers v3 and protoc plugin
# for Go according to: https://grpc.io/docs/quickstart/go/
# See also: https://github.com/protocolbuffers/protobuf/releases

PROTO_FILES=quake.proto quake_api.proto
PROTOS=../../..
INPUT=$(PROTOS)/quake/api/v1
GO_OUT=$(PROTOS)

# default target builds Go stub
all: go_stub

clean:
	rm -f *.pb.go

# Generate Go stubs
# (more about Go plugin: https://github.com/golang/protobuf)
go_stub: quake.pb.go
quake.pb.go: $(PROTO_FILES)
	protoc -I . *.proto --go-grpc_out=. --go_out=.

proxy_stub:
	protoc -I . --grpc-gateway_out . \
	--grpc-gateway_opt logtostderr=true \
	--grpc-gateway_opt paths=source_relative \
	--grpc-gateway_opt generate_unbound_methods=true \
	*.proto