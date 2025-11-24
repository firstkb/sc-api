package tokencoder

import (
	"errors"
	"testing"
	"time"
)

type mockSchema struct {
	marshal   func(testPayload) ([]byte, error)
	unmarshal func([]byte) (testPayload, error)
}

type testPayload struct {
	Value string
	Time  time.Time
}

func (m mockSchema) Marshal(p testPayload) ([]byte, error) {
	if m.marshal != nil {
		return m.marshal(p)
	}
	return []byte(p.Value), nil
}

func (m mockSchema) Unmarshal(data []byte) (testPayload, error) {
	if m.unmarshal != nil {
		return m.unmarshal(data)
	}
	return testPayload{Value: string(data)}, nil
}

func TestCodecEncodeDecode(t *testing.T) {
	codec, err := New([]byte("super-secret"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	payload := testPayload{Value: "hello"}
	schema := mockSchema{}

	token, err := Encode(codec, schema, payload)
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}
	decoded, err := Decode(codec, schema, token)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if decoded.Value != payload.Value {
		t.Fatalf("expected %s got %s", payload.Value, decoded.Value)
	}
}

func TestCodecInvalidSignature(t *testing.T) {
	codec, err := New([]byte("secret"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	token, err := Encode(codec, mockSchema{}, testPayload{Value: "data"})
	if err != nil {
		t.Fatalf("Encode: %v", err)
	}

	// повредим подпись
	token += "tamper"

	if _, err := Decode(codec, mockSchema{}, token); err == nil {
		t.Fatalf("expected error for tampered token")
	}
}

func TestParseKey(t *testing.T) {
	key, err := ParseKey("c2VjcmV0") // base64(secret)
	if err != nil {
		t.Fatalf("ParseKey: %v", err)
	}
	if string(key) != "secret" {
		t.Fatalf("unexpected key %s", string(key))
	}
}

func TestSchemaErrors(t *testing.T) {
	codec, err := New([]byte("secret"))
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	brokenSchema := mockSchema{
		marshal: func(testPayload) ([]byte, error) { return nil, errors.New("boom") },
	}
	if _, err := Encode(codec, brokenSchema, testPayload{}); err == nil {
		t.Fatalf("expected marshal error")
	}
}
