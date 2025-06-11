package display

import "fmt"

// waylandBackend uses native Wayland protocols
// This would require implementing wlr-output-management protocol
// For now, we rely on wlr-randr backend
type waylandBackend struct{}

func newWaylandBackend() (Backend, error) {
	return nil, fmt.Errorf("native Wayland backend not implemented - use wlr-randr")
}

func (w *waylandBackend) GetMonitors() ([]*Monitor, error) {
	return nil, fmt.Errorf("not implemented")
}

func (w *waylandBackend) GetCursorPosition() (x, y int32, err error) {
	return 0, 0, fmt.Errorf("not implemented")
}

func (w *waylandBackend) Close() error {
	return nil
}


// randrBackend uses X11 RandR extension (for XWayland)
type randrBackend struct{}

func newRandrBackend() (Backend, error) {
	// This is actually implemented in portal_backend.go's getMonitorsXRandr
	// We could move it here for better organization
	return &portalBackend{}, nil
}

// sysfsBackend reads from /sys/class/drm
type sysfsBackend struct{}

func newSysfsBackend() (Backend, error) {
	// TODO: Parse /sys/class/drm/card*/modes
	// This gives us connected displays but not positions
	return nil, fmt.Errorf("sysfs backend not implemented yet")
}

func (s *sysfsBackend) GetMonitors() ([]*Monitor, error) {
	return nil, fmt.Errorf("not implemented")
}

func (s *sysfsBackend) GetCursorPosition() (x, y int32, err error) {
	return 0, 0, fmt.Errorf("not implemented")
}

func (s *sysfsBackend) Close() error {
	return nil
}
