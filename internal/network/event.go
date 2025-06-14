package network

import (
	waymonProto "github.com/bnema/waymon/internal/proto"
)

// MouseEvent wraps the protobuf mouse event for internal use
type MouseEvent struct {
	*waymonProto.MouseEvent
}

// EventHandler is a callback for handling mouse events
type EventHandler func(event *MouseEvent) error
