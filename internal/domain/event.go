package domain

// ScrollType represents the type of scroll event.
type ScrollType int

const (
	// ScrollWheel represents a discrete wheel scroll event.
	ScrollWheel ScrollType = iota
	// ScrollFinger represents a touchpad finger scroll.
	ScrollFinger
	// ScrollContinuous represents continuous/smooth scrolling.
	ScrollContinuous
)

// String returns the string representation of ScrollType.
func (s ScrollType) String() string {
	switch s {
	case ScrollWheel:
		return "wheel"
	case ScrollFinger:
		return "finger"
	case ScrollContinuous:
		return "continuous"
	default:
		return "unknown"
	}
}

// ControlEventType represents the type of control event.
type ControlEventType int

const (
	// ControlSwitchToLocal switches control to the local system.
	ControlSwitchToLocal ControlEventType = iota
	// ControlSwitchToClient switches control to a specific client.
	ControlSwitchToClient
	// ControlRequestControl is sent by server to request control of client.
	ControlRequestControl
	// ControlReleaseControl releases control back to local.
	ControlReleaseControl
	// ControlClientListRequest requests the list of connected clients.
	ControlClientListRequest
	// ControlClientListResponse responds with the list of connected clients.
	ControlClientListResponse
	// ControlClientConfig is sent by client with its configuration.
	ControlClientConfig
	// ControlServerShutdown notifies clients of server shutdown.
	ControlServerShutdown
	// ControlPing is a keepalive ping.
	ControlPing
	// ControlPong is a keepalive pong response.
	ControlPong
)

// String returns the string representation of ControlEventType.
func (c ControlEventType) String() string {
	switch c {
	case ControlSwitchToLocal:
		return "switch_to_local"
	case ControlSwitchToClient:
		return "switch_to_client"
	case ControlRequestControl:
		return "request_control"
	case ControlReleaseControl:
		return "release_control"
	case ControlClientListRequest:
		return "client_list_request"
	case ControlClientListResponse:
		return "client_list_response"
	case ControlClientConfig:
		return "client_config"
	case ControlServerShutdown:
		return "server_shutdown"
	case ControlPing:
		return "ping"
	case ControlPong:
		return "pong"
	default:
		return "unknown"
	}
}

// LogLevel represents the severity of a log message.
type LogLevel int

const (
	// LogLevelDebug is for debug messages.
	LogLevelDebug LogLevel = iota
	// LogLevelInfo is for informational messages.
	LogLevelInfo
	// LogLevelWarn is for warning messages.
	LogLevelWarn
	// LogLevelError is for error messages.
	LogLevelError
)

// String returns the string representation of LogLevel.
func (l LogLevel) String() string {
	switch l {
	case LogLevelDebug:
		return "DEBUG"
	case LogLevelInfo:
		return "INFO"
	case LogLevelWarn:
		return "WARN"
	case LogLevelError:
		return "ERROR"
	default:
		return "UNKNOWN"
	}
}

// MouseMoveEvent represents relative mouse movement.
type MouseMoveEvent struct {
	DX float64
	DY float64
}

// MousePositionEvent represents absolute mouse positioning.
type MousePositionEvent struct {
	X int32
	Y int32
}

// MouseButtonEvent represents a mouse button press or release.
type MouseButtonEvent struct {
	Button  uint32
	Pressed bool
}

// MouseScrollEvent represents a scroll/wheel event.
type MouseScrollEvent struct {
	DX   float64
	DY   float64
	Type ScrollType
}

// KeyboardEvent represents a keyboard key press or release.
type KeyboardEvent struct {
	Key       uint32
	Pressed   bool
	Modifiers uint32
	// Character is the Unicode character for semantic keyboard events.
	// When set, the receiver should use this character (translated to appropriate
	// keycodes for the target layout) instead of the raw Key value.
	// This enables proper character transmission across different keyboard layouts.
	Character *rune
}

// LogEvent represents a log message forwarded from client to server.
type LogEvent struct {
	Level          LogLevel
	Message        string
	LoggerName     string
	ClientHostname string
	TimestampMs    int64
}

// ControlEvent represents a control message between server and client.
type ControlEvent struct {
	Type         ControlEventType
	TargetID     string
	ClientConfig *ClientConfig
}

// InputEvent is the main event container that holds one of the possible event types.
// Only one of the event fields will be non-nil at a time.
type InputEvent struct {
	Timestamp int64
	SourceID  string

	// Only one of these will be set
	MouseMove     *MouseMoveEvent
	MousePosition *MousePositionEvent
	MouseButton   *MouseButtonEvent
	MouseScroll   *MouseScrollEvent
	Keyboard      *KeyboardEvent
	Control       *ControlEvent
	Log           *LogEvent
}
