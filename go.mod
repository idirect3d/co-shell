module github.com/idirect3d/co-shell

go 1.25.0

require (
	github.com/idirect3d/co-shell/hub v0.0.0
	github.com/larksuite/oapi-sdk-go/v3 v3.6.1
	github.com/mark3labs/mcp-go v0.8.3
	go.etcd.io/bbolt v1.3.11
	golang.org/x/text v0.36.0
)

require (
	github.com/gogo/protobuf v1.3.2 // indirect
	github.com/gorilla/websocket v1.5.3 // indirect
	github.com/lib/pq v1.10.9 // indirect
	golang.org/x/sys v0.29.0 // indirect
)

replace github.com/idirect3d/co-shell/hub => ./hub
