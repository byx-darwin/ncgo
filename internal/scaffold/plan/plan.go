// Package plan defines the shared machine-readable scaffold plan schema.
package plan

// Item is a machine-readable summary of intended or completed scaffold work.
type Item struct {
	Kind         string `json:"kind"`
	Action       string `json:"action"`
	Path         string `json:"path,omitempty"`
	Detail       string `json:"detail,omitempty"`
	Anchor       string `json:"anchor,omitempty"`
	AnchorSource string `json:"anchorSource,omitempty"`
}
