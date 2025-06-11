package display

import (
	"fmt"
)

// compositorBackend is now simplified since we use wlr-randr as primary
type compositorBackend struct{}

func newCompositorBackend() (Backend, error) {
	// This is now just a placeholder fallback
	// Real display detection happens in wlr-randr backend
	return nil, fmt.Errorf("compositor backend deprecated - use wlr-randr")
}

func (c *compositorBackend) GetMonitors() ([]*Monitor, error) {
	return nil, fmt.Errorf("not implemented - use wlr-randr")
}

func (c *compositorBackend) GetCursorPosition() (x, y int32, err error) {
	return 0, 0, fmt.Errorf("not implemented - use wlr-randr")
}

func (c *compositorBackend) Close() error {
	return nil
}