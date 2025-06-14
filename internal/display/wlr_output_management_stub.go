//go:build !cgo
// +build !cgo

package display

import "fmt"

type wlrOutputManagementBackend struct{}

func newWlrOutputManagementBackend() (Backend, error) {
	return nil, fmt.Errorf("wlr-output-management backend requires CGO")
}

func (w *wlrOutputManagementBackend) GetMonitors() ([]*Monitor, error) {
	return nil, fmt.Errorf("wlr-output-management backend requires CGO")
}

func (w *wlrOutputManagementBackend) GetCursorPosition() (x, y int32, err error) {
	return 0, 0, fmt.Errorf("wlr-output-management backend requires CGO")
}

func (w *wlrOutputManagementBackend) Close() error {
	return nil
}
