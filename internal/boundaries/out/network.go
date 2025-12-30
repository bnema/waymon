package out

import (
	"context"

	"github.com/bnema/waymon/internal/domain"
)

// NetworkServerPort defines the interface for the server-side network transport.
// Implementations handle accepting client connections and sending/receiving events.
type NetworkServerPort interface {
	// Start starts the network server.
	Start(ctx context.Context) error

	// Stop stops the network server and closes all connections.
	Stop()

	// SendEventToClient sends an input event to a specific client by address.
	SendEventToClient(ctx context.Context, clientAddr string, event *domain.InputEvent) error

	// SetMaxClients sets the maximum number of concurrent clients.
	SetMaxClients(maxClients int)

	// Port returns the port the server is listening on.
	Port() int

	// SetOnClientConnected sets the callback for when a client connects.
	// The callback receives the client address and public key fingerprint.
	SetOnClientConnected(callback func(addr, publicKey string))

	// SetOnClientDisconnected sets the callback for when a client disconnects.
	SetOnClientDisconnected(callback func(addr string))

	// SetOnAuthRequest sets the callback for authentication requests.
	// The callback should return true to allow the connection.
	SetOnAuthRequest(callback func(addr, publicKey, fingerprint string) bool)

	// SetOnInputEvent sets the callback for received input events from clients.
	SetOnInputEvent(callback func(event *domain.InputEvent))
}

// NetworkClientPort defines the interface for the client-side network transport.
// Implementations handle connecting to servers and sending/receiving events.
type NetworkClientPort interface {
	// Connect connects to a server at the given address.
	Connect(ctx context.Context, addr string, privateKeyPath string) error

	// Disconnect disconnects from the server.
	Disconnect() error

	// IsConnected returns true if currently connected to a server.
	IsConnected() bool

	// SendEvent sends an input event to the server.
	SendEvent(ctx context.Context, event *domain.InputEvent) error

	// SetOnInputEvent sets the callback for received input events from server.
	SetOnInputEvent(callback func(event *domain.InputEvent))

	// SetOnDisconnected sets the callback for when the connection is lost.
	SetOnDisconnected(callback func(err error))
}
