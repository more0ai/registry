package commsutil

import (
	"fmt"
	"strings"
)

// Default COMMS subjects.
const (
	SubjectRegistry    = "cap.more0.registry.v1"
	SubjectBootstrap   = "system.registry.bootstrap"
	SubjectChangeEvent = "registry.changed"
)

// BuildChangeSubject builds a granular change event subject.
func BuildChangeSubject(app, capability string) string {
	return fmt.Sprintf("registry.changed.%s.%s", app, capability)
}

// BuildCapabilitySubject builds a COMMS subject for a capability.
func BuildCapabilitySubject(app, name string, major int) string {
	safe := strings.ReplaceAll(name, ".", "_")
	return fmt.Sprintf("cap.%s.%s.v%d", app, safe, major)
}
