package commsutil

import (
	"testing"
)

const connectTestPrefix = "commsutil:connect_test"

func TestConnect_InvalidURL(t *testing.T) {
	nc, err := Connect("invalid://not-a-nats-server", "test-client")
	if err == nil {
		if nc != nil {
			nc.Close()
		}
		t.Fatalf("%s - expected error for invalid URL", connectTestPrefix)
	}
	if nc != nil {
		t.Errorf("%s - expected nil connection on error", connectTestPrefix)
	}
}
