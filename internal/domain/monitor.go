package domain

// Monitor represents a display/monitor configuration.
type Monitor struct {
	ID          string
	Name        string
	X           int32
	Y           int32
	Width       int32
	Height      int32
	Primary     bool
	Scale       float64
	RefreshRate int32
}

// DisplayBounds represents the bounding rectangle of all monitors.
type DisplayBounds struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
}

// CursorPosition represents the current cursor position.
type CursorPosition struct {
	X float64
	Y float64
}

// CursorState tracks cursor position and bounds for a client.
type CursorState struct {
	X      float64
	Y      float64
	Bounds DisplayBounds
}
