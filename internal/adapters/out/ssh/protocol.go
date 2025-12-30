// Package ssh provides SSH-based network transport implementations.
package ssh

//go:generate protoc --go_out=. --go_opt=paths=source_relative proto/events.proto

import (
	"fmt"
	"io"

	"github.com/bnema/waymon/internal/adapters/out/ssh/proto"
	"github.com/bnema/waymon/internal/domain"
	pb "google.golang.org/protobuf/proto"
)

// Protocol handles message serialization and wire format.
// Messages are length-prefixed (4 bytes big-endian) followed by protobuf data.

const maxMessageSize = 4096

// WriteMessage writes a protobuf message to the writer with length prefix.
func WriteMessage(w io.Writer, msg pb.Message) error {
	data, err := pb.Marshal(msg)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	length := len(data)
	if length > maxMessageSize {
		return fmt.Errorf("message too large: %d bytes (max %d)", length, maxMessageSize)
	}

	// Write 4-byte big-endian length prefix
	lengthBuf := []byte{
		byte(length >> 24),
		byte(length >> 16),
		byte(length >> 8),
		byte(length),
	}

	if _, err := w.Write(lengthBuf); err != nil {
		return fmt.Errorf("failed to write length prefix: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	// Flush if supported
	if flusher, ok := w.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	}

	return nil
}

// ReadMessage reads a length-prefixed protobuf message from the reader.
func ReadMessage(r io.Reader, msg pb.Message) error {
	// Read 4-byte length prefix
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		return fmt.Errorf("failed to read length prefix: %w", err)
	}

	length := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 | int(lengthBuf[2])<<8 | int(lengthBuf[3])
	if length <= 0 || length > maxMessageSize {
		return fmt.Errorf("invalid message length: %d", length)
	}

	// Read message data
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("failed to read message data: %w", err)
	}

	if err := pb.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return nil
}

// ReadLengthPrefix reads just the length prefix, returning the length.
// Useful for async read patterns.
func ReadLengthPrefix(r io.Reader) (int, error) {
	lengthBuf := make([]byte, 4)
	if _, err := io.ReadFull(r, lengthBuf); err != nil {
		return 0, err
	}

	length := int(lengthBuf[0])<<24 | int(lengthBuf[1])<<16 | int(lengthBuf[2])<<8 | int(lengthBuf[3])
	if length <= 0 || length > maxMessageSize {
		return 0, fmt.Errorf("invalid message length: %d", length)
	}

	return length, nil
}

// ReadMessageData reads the message data after length prefix has been read.
func ReadMessageData(r io.Reader, length int, msg pb.Message) error {
	data := make([]byte, length)
	if _, err := io.ReadFull(r, data); err != nil {
		return fmt.Errorf("failed to read message data: %w", err)
	}

	if err := pb.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return nil
}

// DomainToProto converts a domain.InputEvent to a protobuf InputEvent.
func DomainToProto(event *domain.InputEvent) *proto.InputEvent {
	protoEvent := &proto.InputEvent{
		Timestamp: event.Timestamp,
		SourceId:  event.SourceID,
	}

	switch {
	case event.MouseMove != nil:
		protoEvent.Event = &proto.InputEvent_MouseMove{
			MouseMove: &proto.MouseMoveEvent{
				Dx: event.MouseMove.DX,
				Dy: event.MouseMove.DY,
			},
		}
	case event.MouseButton != nil:
		protoEvent.Event = &proto.InputEvent_MouseButton{
			MouseButton: &proto.MouseButtonEvent{
				Button:  event.MouseButton.Button,
				Pressed: event.MouseButton.Pressed,
			},
		}
	case event.MouseScroll != nil:
		scrollType := proto.ScrollType_SCROLL_WHEEL
		switch event.MouseScroll.Type {
		case domain.ScrollFinger:
			scrollType = proto.ScrollType_SCROLL_FINGER
		case domain.ScrollContinuous:
			scrollType = proto.ScrollType_SCROLL_CONTINUOUS
		}
		protoEvent.Event = &proto.InputEvent_MouseScroll{
			MouseScroll: &proto.MouseScrollEvent{
				Dx:   event.MouseScroll.DX,
				Dy:   event.MouseScroll.DY,
				Type: scrollType,
			},
		}
	case event.Keyboard != nil:
		protoEvent.Event = &proto.InputEvent_Keyboard{
			Keyboard: &proto.KeyboardEvent{
				Key:       event.Keyboard.Key,
				Pressed:   event.Keyboard.Pressed,
				Modifiers: event.Keyboard.Modifiers,
			},
		}
	case event.MousePosition != nil:
		protoEvent.Event = &proto.InputEvent_MousePosition{
			MousePosition: &proto.MousePositionEvent{
				X: event.MousePosition.X,
				Y: event.MousePosition.Y,
			},
		}
	case event.Control != nil:
		controlType := proto.ControlEvent_SWITCH_TO_LOCAL
		switch event.Control.Type {
		case domain.ControlSwitchToClient:
			controlType = proto.ControlEvent_SWITCH_TO_CLIENT
		case domain.ControlRequestControl:
			controlType = proto.ControlEvent_REQUEST_CONTROL
		case domain.ControlReleaseControl:
			controlType = proto.ControlEvent_RELEASE_CONTROL
		case domain.ControlClientListRequest:
			controlType = proto.ControlEvent_CLIENT_LIST_REQUEST
		case domain.ControlClientListResponse:
			controlType = proto.ControlEvent_CLIENT_LIST_RESPONSE
		case domain.ControlClientConfig:
			controlType = proto.ControlEvent_CLIENT_CONFIG
		case domain.ControlServerShutdown:
			controlType = proto.ControlEvent_SERVER_SHUTDOWN
		case domain.ControlPing:
			controlType = proto.ControlEvent_PING
		case domain.ControlPong:
			controlType = proto.ControlEvent_PONG
		}
		protoEvent.Event = &proto.InputEvent_Control{
			Control: &proto.ControlEvent{
				Type:     controlType,
				TargetId: event.Control.TargetID,
			},
		}
	case event.Log != nil:
		logLevel := proto.LogEvent_INFO
		switch event.Log.Level {
		case domain.LogLevelDebug:
			logLevel = proto.LogEvent_DEBUG
		case domain.LogLevelWarn:
			logLevel = proto.LogEvent_WARN
		case domain.LogLevelError:
			logLevel = proto.LogEvent_ERROR
		}
		protoEvent.Event = &proto.InputEvent_Log{
			Log: &proto.LogEvent{
				Level:          logLevel,
				Message:        event.Log.Message,
				LoggerName:     event.Log.LoggerName,
				ClientHostname: event.Log.ClientHostname,
				TimestampMs:    event.Log.TimestampMs,
			},
		}
	}

	return protoEvent
}

// ProtoToDomain converts a protobuf InputEvent to a domain.InputEvent.
func ProtoToDomain(protoEvent *proto.InputEvent) *domain.InputEvent {
	event := &domain.InputEvent{
		Timestamp: protoEvent.Timestamp,
		SourceID:  protoEvent.SourceId,
	}

	switch e := protoEvent.Event.(type) {
	case *proto.InputEvent_MouseMove:
		event.MouseMove = &domain.MouseMoveEvent{
			DX: e.MouseMove.Dx,
			DY: e.MouseMove.Dy,
		}
	case *proto.InputEvent_MouseButton:
		event.MouseButton = &domain.MouseButtonEvent{
			Button:  e.MouseButton.Button,
			Pressed: e.MouseButton.Pressed,
		}
	case *proto.InputEvent_MouseScroll:
		scrollType := domain.ScrollWheel
		switch e.MouseScroll.Type {
		case proto.ScrollType_SCROLL_FINGER:
			scrollType = domain.ScrollFinger
		case proto.ScrollType_SCROLL_CONTINUOUS:
			scrollType = domain.ScrollContinuous
		}
		event.MouseScroll = &domain.MouseScrollEvent{
			DX:   e.MouseScroll.Dx,
			DY:   e.MouseScroll.Dy,
			Type: scrollType,
		}
	case *proto.InputEvent_Keyboard:
		event.Keyboard = &domain.KeyboardEvent{
			Key:       e.Keyboard.Key,
			Pressed:   e.Keyboard.Pressed,
			Modifiers: e.Keyboard.Modifiers,
		}
	case *proto.InputEvent_MousePosition:
		event.MousePosition = &domain.MousePositionEvent{
			X: e.MousePosition.X,
			Y: e.MousePosition.Y,
		}
	case *proto.InputEvent_Control:
		controlType := domain.ControlSwitchToLocal
		switch e.Control.Type {
		case proto.ControlEvent_SWITCH_TO_CLIENT:
			controlType = domain.ControlSwitchToClient
		case proto.ControlEvent_REQUEST_CONTROL:
			controlType = domain.ControlRequestControl
		case proto.ControlEvent_RELEASE_CONTROL:
			controlType = domain.ControlReleaseControl
		case proto.ControlEvent_CLIENT_LIST_REQUEST:
			controlType = domain.ControlClientListRequest
		case proto.ControlEvent_CLIENT_LIST_RESPONSE:
			controlType = domain.ControlClientListResponse
		case proto.ControlEvent_CLIENT_CONFIG:
			controlType = domain.ControlClientConfig
		case proto.ControlEvent_SERVER_SHUTDOWN:
			controlType = domain.ControlServerShutdown
		case proto.ControlEvent_PING:
			controlType = domain.ControlPing
		case proto.ControlEvent_PONG:
			controlType = domain.ControlPong
		}
		event.Control = &domain.ControlEvent{
			Type:     controlType,
			TargetID: e.Control.TargetId,
		}
	case *proto.InputEvent_Log:
		logLevel := domain.LogLevelInfo
		switch e.Log.Level {
		case proto.LogEvent_DEBUG:
			logLevel = domain.LogLevelDebug
		case proto.LogEvent_WARN:
			logLevel = domain.LogLevelWarn
		case proto.LogEvent_ERROR:
			logLevel = domain.LogLevelError
		}
		event.Log = &domain.LogEvent{
			Level:          logLevel,
			Message:        e.Log.Message,
			LoggerName:     e.Log.LoggerName,
			ClientHostname: e.Log.ClientHostname,
			TimestampMs:    e.Log.TimestampMs,
		}
	}

	return event
}

// NewPingEvent creates a new ping control event.
func NewPingEvent(sourceID string) *proto.InputEvent {
	return &proto.InputEvent{
		Event: &proto.InputEvent_Control{
			Control: &proto.ControlEvent{
				Type: proto.ControlEvent_PING,
			},
		},
		SourceId: sourceID,
	}
}

// NewPongEvent creates a new pong control event.
func NewPongEvent(sourceID string) *proto.InputEvent {
	return &proto.InputEvent{
		Event: &proto.InputEvent_Control{
			Control: &proto.ControlEvent{
				Type: proto.ControlEvent_PONG,
			},
		},
		SourceId: sourceID,
	}
}

// IsPing checks if the event is a ping control event.
func IsPing(event *proto.InputEvent) bool {
	if ctrl, ok := event.Event.(*proto.InputEvent_Control); ok {
		return ctrl.Control.Type == proto.ControlEvent_PING
	}
	return false
}

// IsPong checks if the event is a pong control event.
func IsPong(event *proto.InputEvent) bool {
	if ctrl, ok := event.Event.(*proto.InputEvent_Control); ok {
		return ctrl.Control.Type == proto.ControlEvent_PONG
	}
	return false
}
