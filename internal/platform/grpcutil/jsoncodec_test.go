package grpcutil

import "testing"

func TestJSONCodecRoundTrip(t *testing.T) {
	t.Parallel()

	codec := JSONCodec{}
	if codec.Name() != "json" {
		t.Fatalf("expected codec name json, got %q", codec.Name())
	}

	type payload struct {
		Message string `json:"message"`
	}

	data, err := codec.Marshal(payload{Message: "hello"})
	if err != nil {
		t.Fatalf("Marshal returned error: %v", err)
	}

	var decoded payload
	if err := codec.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal returned error: %v", err)
	}
	if decoded.Message != "hello" {
		t.Fatalf("expected decoded message hello, got %q", decoded.Message)
	}
}
