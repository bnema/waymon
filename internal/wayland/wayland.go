package wayland

import (
	"context"
	"fmt"
	"time"

	"github.com/ThomasT75/uinput"
	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
	"github.com/rajveermalviya/go-wayland/wayland/client"
)

// WaylandClient represents a Wayland client for input capture and injection
type WaylandClient struct {
	display   *client.Display
	registry  *client.Registry
	connected bool

	// Display and cursor tracking
	outputs map[uint32]*OutputInfo
	seats   map[uint32]*SeatInfo
	cursorX int32
	cursorY int32
}

// OutputInfo contains information about a Wayland output (monitor)
type OutputInfo struct {
	ID          uint32
	Name        string
	Description string
	X           int32
	Y           int32
	Width       int32
	Height      int32
	Scale       int32
	Transform   int32
}

// SeatInfo contains information about a Wayland seat (input device group)
type SeatInfo struct {
	ID          uint32
	Name        string
	HasPointer  bool
	HasKeyboard bool
	HasTouch    bool
}

// InputCapture interface defines methods for capturing input from Wayland (server-side)
type InputCapture interface {
	Start(ctx context.Context) error
	Stop() error
	SetTarget(clientID string) error
	OnInputEvent(func(*protocol.InputEvent))
}

// InputInjector interface defines methods for injecting input to Wayland (client-side)
type InputInjector interface {
	InjectInputEvent(*protocol.InputEvent) error
	Connect() error
	Disconnect()
}

// WaylandInputCapture implements InputCapture for server-side input capture
type WaylandInputCapture struct {
	client          *WaylandClient
	onInputEvent    func(*protocol.InputEvent)
	currentClientID string
	capturing       bool
}

// WaylandInputInjector implements InputInjector for client-side input injection
type WaylandInputInjector struct {
	client   *WaylandClient
	mouse    uinput.Mouse
	keyboard uinput.Keyboard
}

// NewWaylandClient creates a new Wayland client
func NewWaylandClient() (*WaylandClient, error) {
	return &WaylandClient{
		connected: false,
		outputs:   make(map[uint32]*OutputInfo),
		seats:     make(map[uint32]*SeatInfo),
	}, nil
}

// Connect establishes connection to Wayland display
func (w *WaylandClient) Connect() error {
	display, err := client.Connect("")
	if err != nil {
		return fmt.Errorf("failed to connect to Wayland display: %w", err)
	}

	w.display = display
	w.connected = true

	// Get the registry to discover global objects
	registry, err := display.GetRegistry()
	if err != nil {
		w.Disconnect()
		return fmt.Errorf("failed to get registry: %w", err)
	}

	w.registry = registry
	return nil
}

// Disconnect closes the Wayland connection
func (w *WaylandClient) Disconnect() {
	if w.display != nil {
		w.display.Destroy()
		w.display = nil
		w.registry = nil
	}
	w.connected = false
}

// IsConnected returns whether the client is connected to Wayland
func (w *WaylandClient) IsConnected() bool {
	return w.connected
}

// NewInputCapture creates a new input capture instance for server-side use
func (w *WaylandClient) NewInputCapture() InputCapture {
	return &WaylandInputCapture{
		client:    w,
		capturing: false,
	}
}

// NewInputInjector creates a new input injector instance for client-side use
func (w *WaylandClient) NewInputInjector() InputInjector {
	return &WaylandInputInjector{
		client: w,
	}
}

// InputCapture implementation

func (w *WaylandInputCapture) Start(ctx context.Context) error {
	if !w.client.IsConnected() {
		return fmt.Errorf("wayland client not connected")
	}

	w.capturing = true

	logger.Debug("Started Wayland input capture (test mode)")

	// Start a test input simulator for testing the event flow
	go w.simulateInputEvents(ctx)

	// TODO: Implement proper Wayland input capture using:
	// - zwp_relative_pointer_v1 for mouse capture
	// - zwp_pointer_constraints_v1 for pointer confinement
	// - seat listeners for keyboard/pointer events

	return nil
}

func (w *WaylandInputCapture) Stop() error {
	w.capturing = false
	logger.Debug("Stopped Wayland input capture")
	return nil
}

func (w *WaylandInputCapture) SetTarget(clientID string) error {
	w.currentClientID = clientID
	logger.Infof("Set input capture target to client: %s", clientID)
	return nil
}

func (w *WaylandInputCapture) OnInputEvent(callback func(*protocol.InputEvent)) {
	w.onInputEvent = callback
}

// InputInjector implementation

func (w *WaylandInputInjector) Connect() error {
	// Create virtual mouse device
	mouse, err := uinput.CreateMouse("/dev/uinput", []byte("Waymon Virtual Mouse"))
	if err != nil {
		return fmt.Errorf("failed to create virtual mouse: %w", err)
	}
	w.mouse = mouse

	// Create virtual keyboard device
	keyboard, err := uinput.CreateKeyboard("/dev/uinput", []byte("Waymon Virtual Keyboard"))
	if err != nil {
		if w.mouse != nil {
			w.mouse.Close()
		}
		return fmt.Errorf("failed to create virtual keyboard: %w", err)
	}
	w.keyboard = keyboard

	// Connect to Wayland (optional, for future extensions)
	if err := w.client.Connect(); err != nil {
		logger.Warnf("Could not connect to Wayland display: %v", err)
		// Don't fail here since uinput doesn't require Wayland connection
	}

	return nil
}

func (w *WaylandInputInjector) Disconnect() {
	if w.mouse != nil {
		w.mouse.Close()
		w.mouse = nil
	}
	if w.keyboard != nil {
		w.keyboard.Close()
		w.keyboard = nil
	}
	w.client.Disconnect()
}

func (w *WaylandInputInjector) InjectInputEvent(event *protocol.InputEvent) error {
	if w.mouse == nil || w.keyboard == nil {
		return fmt.Errorf("input devices not initialized")
	}

	switch e := event.Event.(type) {
	case *protocol.InputEvent_MouseMove:
		return w.injectMouseMove(e.MouseMove.Dx, e.MouseMove.Dy)
	case *protocol.InputEvent_MousePosition:
		return w.injectMousePosition(e.MousePosition.X, e.MousePosition.Y)
	case *protocol.InputEvent_MouseButton:
		return w.injectMouseButton(e.MouseButton.Button, e.MouseButton.Pressed)
	case *protocol.InputEvent_MouseScroll:
		return w.injectMouseScroll(e.MouseScroll.Dx, e.MouseScroll.Dy)
	case *protocol.InputEvent_Keyboard:
		return w.injectKeyboard(e.Keyboard.Key, e.Keyboard.Pressed, e.Keyboard.Modifiers)
	case *protocol.InputEvent_Control:
		return w.handleControlEvent(e.Control)
	default:
		return fmt.Errorf("unsupported input event type")
	}
}

func (w *WaylandInputInjector) injectMouseMove(dx, dy float64) error {
	return w.mouse.Move(int32(dx), int32(dy))
}

func (w *WaylandInputInjector) injectMousePosition(x, y int32) error {
	// For absolute positioning, we need to calculate the delta from current position
	// Since uinput only supports relative movement, we'll calculate the delta
	currentX, currentY := w.client.GetCursorPosition()

	// Calculate delta to reach target position
	deltaX := x - currentX
	deltaY := y - currentY

	// Update client's tracked cursor position
	w.client.SetCursorPosition(x, y)

	// Apply the movement
	if deltaX != 0 || deltaY != 0 {
		logger.Debugf("Positioning cursor at (%d, %d), moving by delta (%d, %d)", x, y, deltaX, deltaY)
		return w.mouse.Move(deltaX, deltaY)
	}

	return nil
}

func (w *WaylandInputInjector) injectMouseButton(button uint32, pressed bool) error {
	switch button {
	case 1: // Left button
		if pressed {
			return w.mouse.LeftPress()
		} else {
			return w.mouse.LeftRelease()
		}
	case 2: // Middle button
		if pressed {
			return w.mouse.MiddlePress()
		} else {
			return w.mouse.MiddleRelease()
		}
	case 3: // Right button
		if pressed {
			return w.mouse.RightPress()
		} else {
			return w.mouse.RightRelease()
		}
	default:
		return fmt.Errorf("unsupported mouse button: %d", button)
	}
}

func (w *WaylandInputInjector) injectMouseScroll(dx, dy float64) error {
	// Handle vertical scrolling
	if dy != 0 {
		if err := w.mouse.Wheel(false, int32(dy)); err != nil {
			return err
		}
	}

	// Handle horizontal scrolling
	if dx != 0 {
		if err := w.mouse.Wheel(true, int32(dx)); err != nil {
			return err
		}
	}

	return nil
}

func (w *WaylandInputInjector) injectKeyboard(key uint32, pressed bool, modifiers uint32) error {
	// Convert the key code to uinput key code
	// Note: This assumes Linux input event codes are being used
	// You may need to add key mapping logic here if needed

	if pressed {
		return w.keyboard.KeyDown(int(key))
	} else {
		return w.keyboard.KeyUp(int(key))
	}
}

func (w *WaylandInputInjector) handleControlEvent(control *protocol.ControlEvent) error {
	logger.Debugf("Handling control event: %v", control.Type)
	// Control events are typically handled at a higher level
	return nil
}

// Display and cursor tracking methods

// GetOutputs returns information about all available outputs (monitors)
func (w *WaylandClient) GetOutputs() map[uint32]*OutputInfo {
	return w.outputs
}

// GetPrimaryOutput returns the primary output, or nil if none found
func (w *WaylandClient) GetPrimaryOutput() *OutputInfo {
	// For now, return the first output as primary
	// In a full implementation, we'd track which output is marked as primary
	for _, output := range w.outputs {
		return output
	}
	return nil
}

// GetCursorPosition returns the current cursor position
func (w *WaylandClient) GetCursorPosition() (int32, int32) {
	return w.cursorX, w.cursorY
}

// GetSeats returns information about all available seats
func (w *WaylandClient) GetSeats() map[uint32]*SeatInfo {
	return w.seats
}

// IsAtScreenEdge checks if the cursor is at a screen edge
func (w *WaylandClient) IsAtScreenEdge(threshold int32) (bool, string) {
	for _, output := range w.outputs {
		// Check if cursor is within this output bounds
		if w.cursorX >= output.X && w.cursorX < output.X+output.Width &&
			w.cursorY >= output.Y && w.cursorY < output.Y+output.Height {

			// Calculate relative position within this output
			relX := w.cursorX - output.X
			relY := w.cursorY - output.Y

			// Check edges
			if relX <= threshold {
				return true, "left"
			}
			if relX >= output.Width-threshold {
				return true, "right"
			}
			if relY <= threshold {
				return true, "top"
			}
			if relY >= output.Height-threshold {
				return true, "bottom"
			}
		}
	}
	return false, ""
}

// SetCursorPosition updates the tracked cursor position
// This would be called by pointer motion events
func (w *WaylandClient) SetCursorPosition(x, y int32) {
	w.cursorX = x
	w.cursorY = y
}

// simulateInputEvents generates test input events for testing the event flow
func (w *WaylandInputCapture) simulateInputEvents(ctx context.Context) {
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	eventCount := 0
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !w.capturing || w.onInputEvent == nil {
				continue
			}

			// Only send events if we have an active client target
			if w.currentClientID == "" {
				logger.Debug("No active client target, skipping simulated event")
				continue
			}

			eventCount++
			timestamp := time.Now().UnixNano()

			// Simulate different types of input events
			switch eventCount % 4 {
			case 0:
				// Mouse move event
				event := &protocol.InputEvent{
					Event: &protocol.InputEvent_MouseMove{
						MouseMove: &protocol.MouseMoveEvent{
							Dx: 10.0,
							Dy: 5.0,
						},
					},
					Timestamp: timestamp,
					SourceId:  "test-server",
				}
				logger.Debugf("Simulating mouse move event to client %s", w.currentClientID)
				w.onInputEvent(event)

			case 1:
				// Mouse button press
				event := &protocol.InputEvent{
					Event: &protocol.InputEvent_MouseButton{
						MouseButton: &protocol.MouseButtonEvent{
							Button:  1, // Left button
							Pressed: true,
						},
					},
					Timestamp: timestamp,
					SourceId:  "test-server",
				}
				logger.Debugf("Simulating mouse button press to client %s", w.currentClientID)
				w.onInputEvent(event)

			case 2:
				// Mouse button release
				event := &protocol.InputEvent{
					Event: &protocol.InputEvent_MouseButton{
						MouseButton: &protocol.MouseButtonEvent{
							Button:  1, // Left button
							Pressed: false,
						},
					},
					Timestamp: timestamp,
					SourceId:  "test-server",
				}
				logger.Debugf("Simulating mouse button release to client %s", w.currentClientID)
				w.onInputEvent(event)

			case 3:
				// Keyboard event
				event := &protocol.InputEvent{
					Event: &protocol.InputEvent_Keyboard{
						Keyboard: &protocol.KeyboardEvent{
							Key:       65, // 'A' key
							Pressed:   true,
							Modifiers: 0,
						},
					},
					Timestamp: timestamp,
					SourceId:  "test-server",
				}
				logger.Debugf("Simulating keyboard event to client %s", w.currentClientID)
				w.onInputEvent(event)
			}
		}
	}
}

// RegisterOutput adds or updates output information
func (w *WaylandClient) RegisterOutput(id uint32, info *OutputInfo) {
	w.outputs[id] = info
	logger.Debugf("Registered output %d: %s (%dx%d at %d,%d)",
		id, info.Name, info.Width, info.Height, info.X, info.Y)
}

// UnregisterOutput removes output information
func (w *WaylandClient) UnregisterOutput(id uint32) {
	if output, exists := w.outputs[id]; exists {
		logger.Debugf("Unregistered output %d: %s", id, output.Name)
		delete(w.outputs, id)
	}
}

// RegisterSeat adds or updates seat information
func (w *WaylandClient) RegisterSeat(id uint32, info *SeatInfo) {
	w.seats[id] = info
	logger.Debugf("Registered seat %d: %s (pointer: %v, keyboard: %v)",
		id, info.Name, info.HasPointer, info.HasKeyboard)
}

// UnregisterSeat removes seat information
func (w *WaylandClient) UnregisterSeat(id uint32) {
	if seat, exists := w.seats[id]; exists {
		logger.Debugf("Unregistered seat %d: %s", id, seat.Name)
		delete(w.seats, id)
	}
}
