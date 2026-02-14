// Package events defines event types and publisher interfaces for registry change events.
package events

// RegistryChangedEvent is emitted when a capability's registry entry changes.
type RegistryChangedEvent struct {
	App             string   `json:"app"`
	Capability      string   `json:"capability"`
	ChangedFields   []string `json:"changedFields"`
	NewDefaultMajor *int     `json:"newDefaultMajor,omitempty"`
	AffectedMajors  []int    `json:"affectedMajors"`
	Revision        int      `json:"revision"`
	Etag            string   `json:"etag"`
	Timestamp       string   `json:"timestamp"`
	Env             string   `json:"env,omitempty"`
}
