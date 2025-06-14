//go:build !cgo
// +build !cgo

package display

import "fmt"

// wlrCgoBackend stub for when CGO is disabled
type wlrCgoBackend struct{}

func newWlrCgoBackend() (Backend, error) {
	return nil, fmt.Errorf("CGO backend not available (build with CGO enabled)")
}

func (w *wlrCgoBackend) GetMonitors() ([]*Monitor, error) {
	return nil, fmt.Errorf("not implemented")
}

func (w *wlrCgoBackend) GetCursorPosition() (x, y int32, err error) {
	return 0, 0, fmt.Errorf("not implemented")
}

func (w *wlrCgoBackend) Close() error {
	return nil
}
