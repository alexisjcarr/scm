package grpcutil

import "encoding/json"

// JSONCodec keeps the MVP buildable without requiring generated protobuf code
// during every edit cycle. The transport still uses gRPC, and the repository
// keeps the canonical protobuf contracts under /proto.
type JSONCodec struct{}

func (JSONCodec) Name() string {
	return "json"
}

func (JSONCodec) Marshal(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (JSONCodec) Unmarshal(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
