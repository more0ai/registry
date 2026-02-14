package commsutil

import "encoding/json"

// EncodePayload serializes a value to JSON bytes.
func EncodePayload(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

// DecodePayload deserializes JSON bytes into the given target.
func DecodePayload(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}
