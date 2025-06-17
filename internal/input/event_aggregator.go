package input

import (
	"context"
	"sync"
	"time"

	"github.com/bnema/waymon/internal/logger"
	"github.com/bnema/waymon/internal/protocol"
)

// EventAggregator collects and filters input events from multiple sources
type EventAggregator struct {
	mu           sync.RWMutex
	eventChan    chan *protocol.InputEvent
	filteredChan chan *protocol.InputEvent
	ctx          context.Context
	cancel       context.CancelFunc

	// Configuration
	mouseSensitivity float64
	scrollSpeed      float64
	enableKeyboard   bool

	// Event filtering and aggregation
	mouseAccumulator MouseAccumulator
	eventFilters     []EventFilter
}

// MouseAccumulator accumulates mouse movement events
type MouseAccumulator struct {
	mu       sync.Mutex
	deltaX   float64
	deltaY   float64
	lastSent time.Time
}

// EventFilter represents a filter that can process events
type EventFilter interface {
	ProcessEvent(event *protocol.InputEvent) *protocol.InputEvent
}

// NewEventAggregator creates a new event aggregator
func NewEventAggregator() *EventAggregator {
	return &EventAggregator{
		eventChan:        make(chan *protocol.InputEvent, 1000), // Increased buffer
		filteredChan:     make(chan *protocol.InputEvent, 500),  // Increased buffer
		mouseSensitivity: 1.0,
		scrollSpeed:      1.0,
		enableKeyboard:   true,
	}
}

// Start starts the event aggregator
func (ea *EventAggregator) Start(ctx context.Context) error {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	ea.ctx, ea.cancel = context.WithCancel(ctx)

	// Start event processing
	go ea.processEvents()
	go ea.flushMouseMovements()

	logger.Debug("Event aggregator started")
	return nil
}

// Stop stops the event aggregator
func (ea *EventAggregator) Stop() error {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	if ea.cancel != nil {
		ea.cancel()
	}

	// Close channels
	close(ea.eventChan)
	close(ea.filteredChan)

	logger.Debug("Event aggregator stopped")
	return nil
}

// SetConfig sets the aggregator configuration
func (ea *EventAggregator) SetConfig(mouseSensitivity, scrollSpeed float64, enableKeyboard bool) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	ea.mouseSensitivity = mouseSensitivity
	ea.scrollSpeed = scrollSpeed
	ea.enableKeyboard = enableKeyboard
}

// AddEventFilter adds an event filter
func (ea *EventAggregator) AddEventFilter(filter EventFilter) {
	ea.mu.Lock()
	defer ea.mu.Unlock()

	ea.eventFilters = append(ea.eventFilters, filter)
}

// InputChannel returns the channel for receiving raw input events
func (ea *EventAggregator) InputChannel() chan<- *protocol.InputEvent {
	return ea.eventChan
}

// OutputChannel returns the channel for reading filtered events
func (ea *EventAggregator) OutputChannel() <-chan *protocol.InputEvent {
	return ea.filteredChan
}

// processEvents processes incoming events and applies filters
func (ea *EventAggregator) processEvents() {
	defer func() {
		if r := recover(); r != nil {
			logger.Errorf("Event aggregator panic: %v", r)
		}
	}()

	for {
		select {
		case <-ea.ctx.Done():
			return
		case event, ok := <-ea.eventChan:
			if !ok {
				return
			}

			// Process the event
			processedEvent := ea.processEvent(event)
			if processedEvent != nil {
				// Send to output channel
				select {
				case ea.filteredChan <- processedEvent:
				default:
					// Channel full, drop event
					logger.Warnf("Filtered event channel full, dropping event")
				}
			}
		}
	}
}

// processEvent processes a single event
func (ea *EventAggregator) processEvent(event *protocol.InputEvent) *protocol.InputEvent {
	ea.mu.RLock()
	defer ea.mu.RUnlock()

	// Apply sensitivity and speed settings
	switch e := event.Event.(type) {
	case *protocol.InputEvent_MouseMove:
		// Accumulate mouse movements instead of sending immediately
		ea.mouseAccumulator.mu.Lock()
		ea.mouseAccumulator.deltaX += e.MouseMove.Dx * ea.mouseSensitivity
		ea.mouseAccumulator.deltaY += e.MouseMove.Dy * ea.mouseSensitivity
		ea.mouseAccumulator.mu.Unlock()

		// Don't send the event immediately, let flushMouseMovements handle it
		return nil

	case *protocol.InputEvent_MouseScroll:
		// Apply scroll speed
		e.MouseScroll.Dx *= ea.scrollSpeed
		e.MouseScroll.Dy *= ea.scrollSpeed

	case *protocol.InputEvent_Keyboard:
		// Filter keyboard events if disabled
		if !ea.enableKeyboard {
			return nil
		}
	}

	// Apply custom filters
	processedEvent := event
	for _, filter := range ea.eventFilters {
		processedEvent = filter.ProcessEvent(processedEvent)
		if processedEvent == nil {
			break
		}
	}

	return processedEvent
}

// flushMouseMovements periodically flushes accumulated mouse movements
func (ea *EventAggregator) flushMouseMovements() {
	ticker := time.NewTicker(16 * time.Millisecond) // ~60 FPS
	defer ticker.Stop()

	for {
		select {
		case <-ea.ctx.Done():
			return
		case <-ticker.C:
			ea.mouseAccumulator.mu.Lock()

			if ea.mouseAccumulator.deltaX != 0 || ea.mouseAccumulator.deltaY != 0 {
				event := &protocol.InputEvent{
					Event: &protocol.InputEvent_MouseMove{
						MouseMove: &protocol.MouseMoveEvent{
							Dx: ea.mouseAccumulator.deltaX,
							Dy: ea.mouseAccumulator.deltaY,
						},
					},
					Timestamp: time.Now().UnixNano(),
					SourceId:  "event-aggregator",
				}

				// Reset accumulator
				ea.mouseAccumulator.deltaX = 0
				ea.mouseAccumulator.deltaY = 0
				ea.mouseAccumulator.lastSent = time.Now()

				// Send the accumulated movement
				select {
				case ea.filteredChan <- event:
				default:
					logger.Warnf("Filtered event channel full, dropping mouse movement")
				}
			}

			ea.mouseAccumulator.mu.Unlock()
		}
	}
}

// DeduplicationFilter removes duplicate events
type DeduplicationFilter struct {
	lastEvent     *protocol.InputEvent
	lastTimestamp time.Time
}

// NewDeduplicationFilter creates a new deduplication filter
func NewDeduplicationFilter() *DeduplicationFilter {
	return &DeduplicationFilter{}
}

// ProcessEvent processes an event through the deduplication filter
func (df *DeduplicationFilter) ProcessEvent(event *protocol.InputEvent) *protocol.InputEvent {
	now := time.Now()

	// Skip duplicate events within a short time window
	if df.lastEvent != nil && df.eventsSimilar(df.lastEvent, event) &&
		now.Sub(df.lastTimestamp) < 5*time.Millisecond {
		return nil
	}

	df.lastEvent = event
	df.lastTimestamp = now
	return event
}

// eventsSimilar checks if two events are similar enough to be considered duplicates
func (df *DeduplicationFilter) eventsSimilar(e1, e2 *protocol.InputEvent) bool {
	switch event1 := e1.Event.(type) {
	case *protocol.InputEvent_MouseMove:
		if event2, ok := e2.Event.(*protocol.InputEvent_MouseMove); ok {
			// Consider movements similar if they're very small
			return abs(event1.MouseMove.Dx) < 1 && abs(event1.MouseMove.Dy) < 1 &&
				abs(event2.MouseMove.Dx) < 1 && abs(event2.MouseMove.Dy) < 1
		}
	case *protocol.InputEvent_MouseButton:
		if event2, ok := e2.Event.(*protocol.InputEvent_MouseButton); ok {
			return event1.MouseButton.Button == event2.MouseButton.Button &&
				event1.MouseButton.Pressed == event2.MouseButton.Pressed
		}
	case *protocol.InputEvent_Keyboard:
		if event2, ok := e2.Event.(*protocol.InputEvent_Keyboard); ok {
			return event1.Keyboard.Key == event2.Keyboard.Key &&
				event1.Keyboard.Pressed == event2.Keyboard.Pressed
		}
	}
	return false
}

// RateLimitFilter limits the rate of events
type RateLimitFilter struct {
	lastSent      time.Time
	minInterval   time.Duration
	eventTypeLast map[string]time.Time
}

// NewRateLimitFilter creates a new rate limiting filter
func NewRateLimitFilter(minInterval time.Duration) *RateLimitFilter {
	return &RateLimitFilter{
		minInterval:   minInterval,
		eventTypeLast: make(map[string]time.Time),
	}
}

// ProcessEvent processes an event through the rate limiting filter
func (rl *RateLimitFilter) ProcessEvent(event *protocol.InputEvent) *protocol.InputEvent {
	now := time.Now()
	eventType := getEventTypeName(event)

	if lastSent, exists := rl.eventTypeLast[eventType]; exists {
		if now.Sub(lastSent) < rl.minInterval {
			return nil // Rate limited
		}
	}

	rl.eventTypeLast[eventType] = now
	return event
}

// getEventTypeName returns a string representation of the event type
func getEventTypeName(event *protocol.InputEvent) string {
	switch event.Event.(type) {
	case *protocol.InputEvent_MouseMove:
		return "mouse_move"
	case *protocol.InputEvent_MouseButton:
		return "mouse_button"
	case *protocol.InputEvent_MouseScroll:
		return "mouse_scroll"
	case *protocol.InputEvent_Keyboard:
		return "keyboard"
	default:
		return "unknown"
	}
}

// abs returns the absolute value of a float64
func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
