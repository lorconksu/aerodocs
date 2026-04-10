module github.com/wyiu/aerodocs/agent

go 1.26.2

replace github.com/wyiu/aerodocs/proto => ../proto

require (
	github.com/wyiu/aerodocs/proto v0.0.0-00010101000000-000000000000
	golang.org/x/sys v0.42.0
)

require (
	golang.org/x/net v0.48.0 // indirect
	golang.org/x/text v0.32.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20251202230838-ff82c1b0f217 // indirect
	google.golang.org/grpc v1.79.3 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)
