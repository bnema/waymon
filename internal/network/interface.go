package network

import (
	"context"
)

// Client defines the interface for network clients
type Client interface {
	Connect(ctx context.Context, serverAddr string) error
	Disconnect() error
	IsConnected() bool
	Reconnect(ctx context.Context, serverAddr string) error
	SendMouseEvent(event *MouseEvent) error
	SendMouseBatch(events []*MouseEvent) error
}