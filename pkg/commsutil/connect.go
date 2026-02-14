// Package commsutil provides COMMS connection helpers and utilities.
package commsutil

import (
	"fmt"
	"log/slog"
	"time"

	comms "github.com/nats-io/nats.go"
)

const logPrefix = "commsutil:connect"

// Connect creates a COMMS connection to the given URL.
func Connect(url, name string) (*comms.Conn, error) {
	slog.Info(fmt.Sprintf("%s - Connecting to COMMS at %s as %s", logPrefix, url, name))

	nc, err := comms.Connect(url,
		comms.Name(name),
		comms.Timeout(10*time.Second),
		comms.ReconnectWait(2*time.Second),
		comms.MaxReconnects(60),
		comms.DisconnectErrHandler(func(_ *comms.Conn, err error) {
			slog.Warn(fmt.Sprintf("%s - COMMS disconnected: %v", logPrefix, err))
		}),
		comms.ReconnectHandler(func(nc *comms.Conn) {
			slog.Info(fmt.Sprintf("%s - COMMS reconnected to %s", logPrefix, nc.ConnectedUrl()))
		}),
		comms.ClosedHandler(func(nc *comms.Conn) {
			slog.Info(fmt.Sprintf("%s - COMMS connection closed", logPrefix))
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("%s - failed to connect to COMMS: %w", logPrefix, err)
	}

	slog.Info(fmt.Sprintf("%s - Connected to COMMS at %s", logPrefix, nc.ConnectedUrl()))
	return nc, nil
}
