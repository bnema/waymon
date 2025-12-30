// Package components provides reusable TUI components.
package components

import (
	"fmt"
	"math"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/bnema/waymon/internal/adapters/in/tui/styles"
	"github.com/bnema/waymon/internal/domain"
)

// MonitorDisplay renders a visual representation of monitor layout.
type MonitorDisplay struct {
	monitors       []domain.Monitor
	cursorPos      *domain.CursorPosition
	width          int
	height         int
	showLabels     bool
	showCursor     bool
	highlightedID  string
	scale          float64
	clientName     string
	showDimensions bool
}

// NewMonitorDisplay creates a new monitor display component.
func NewMonitorDisplay() MonitorDisplay {
	return MonitorDisplay{
		monitors:       make([]domain.Monitor, 0),
		width:          60,
		height:         20,
		showLabels:     true,
		showCursor:     true,
		scale:          0,
		showDimensions: true,
	}
}

// WithMonitors sets the monitors to display.
func (m MonitorDisplay) WithMonitors(monitors []domain.Monitor) MonitorDisplay {
	m.monitors = monitors
	return m
}

// WithCursorPosition sets the cursor position to show.
func (m MonitorDisplay) WithCursorPosition(pos *domain.CursorPosition) MonitorDisplay {
	m.cursorPos = pos
	return m
}

// WithSize sets the display area dimensions.
func (m MonitorDisplay) WithSize(width, height int) MonitorDisplay {
	m.width = width
	m.height = height
	return m
}

// WithShowLabels sets whether to show monitor labels.
func (m MonitorDisplay) WithShowLabels(show bool) MonitorDisplay {
	m.showLabels = show
	return m
}

// WithShowCursor sets whether to show cursor position.
func (m MonitorDisplay) WithShowCursor(show bool) MonitorDisplay {
	m.showCursor = show
	return m
}

// WithHighlighted sets the highlighted monitor ID.
func (m MonitorDisplay) WithHighlighted(id string) MonitorDisplay {
	m.highlightedID = id
	return m
}

// WithClientName sets the client name to display in the title.
func (m MonitorDisplay) WithClientName(name string) MonitorDisplay {
	m.clientName = name
	return m
}

// WithShowDimensions sets whether to show monitor dimensions.
func (m MonitorDisplay) WithShowDimensions(show bool) MonitorDisplay {
	m.showDimensions = show
	return m
}

// View renders the monitor display.
func (m MonitorDisplay) View() string {
	if len(m.monitors) == 0 {
		return m.renderEmpty()
	}

	var lines []string

	// Title
	title := m.renderTitle()
	if title != "" {
		lines = append(lines, title, "")
	}

	// Monitor grid
	grid := m.renderGrid()
	lines = append(lines, grid)

	// Legend
	if m.showLabels {
		lines = append(lines, "", m.renderLegend())
	}

	return lipgloss.JoinVertical(lipgloss.Left, lines...)
}

// renderTitle renders the title section.
func (m MonitorDisplay) renderTitle() string {
	if m.clientName == "" {
		return styles.TitleStyle.Render(
			fmt.Sprintf("%s Monitor Layout", styles.IconMonitor),
		)
	}
	return styles.TitleStyle.Render(
		fmt.Sprintf("%s %s - Monitor Layout", styles.IconMonitor, m.clientName),
	)
}

// renderEmpty renders the empty state.
func (m MonitorDisplay) renderEmpty() string {
	emptyStyle := styles.MutedStyle.
		Width(m.width).
		Align(lipgloss.Center).
		Padding(2, 0)

	return emptyStyle.Render(
		styles.IconInfo + " No monitors detected\n\n" +
			"Waiting for monitor information...",
	)
}

// renderGrid renders the ASCII grid representation of monitors.
func (m MonitorDisplay) renderGrid() string {
	// Calculate bounds
	bounds := m.calculateBounds()
	if bounds.MaxX <= bounds.MinX || bounds.MaxY <= bounds.MinY {
		return ""
	}

	// Calculate scale factor
	displayWidth := bounds.MaxX - bounds.MinX
	displayHeight := bounds.MaxY - bounds.MinY

	// Use provided scale or calculate auto-scale
	scaleX := float64(m.width-4) / displayWidth   // Leave margin for borders
	scaleY := float64(m.height-4) / displayHeight // Leave margin for borders

	// Use the smaller scale to fit both dimensions
	scale := math.Min(scaleX, scaleY)
	if m.scale > 0 {
		scale = m.scale
	}

	// Create canvas
	canvasWidth := int(displayWidth*scale) + 2
	canvasHeight := int(displayHeight*scale) + 2
	if canvasWidth < 10 {
		canvasWidth = 10
	}
	if canvasHeight < 5 {
		canvasHeight = 5
	}

	canvas := make([][]rune, canvasHeight)
	for i := range canvas {
		canvas[i] = make([]rune, canvasWidth)
		for j := range canvas[i] {
			canvas[i][j] = ' '
		}
	}

	// Draw monitors
	for i, mon := range m.monitors {
		m.drawMonitor(canvas, mon, bounds, scale, i)
	}

	// Draw cursor if available
	if m.showCursor && m.cursorPos != nil {
		m.drawCursor(canvas, bounds, scale)
	}

	// Convert canvas to string
	var lines []string
	for _, row := range canvas {
		lines = append(lines, string(row))
	}

	// Apply box style
	content := strings.Join(lines, "\n")
	boxStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderForeground(styles.Border).
		Padding(0, 1)

	return boxStyle.Render(content)
}

// calculateBounds calculates the bounding box of all monitors.
func (m MonitorDisplay) calculateBounds() domain.DisplayBounds {
	if len(m.monitors) == 0 {
		return domain.DisplayBounds{}
	}

	bounds := domain.DisplayBounds{
		MinX: float64(m.monitors[0].X),
		MinY: float64(m.monitors[0].Y),
		MaxX: float64(m.monitors[0].X + m.monitors[0].Width),
		MaxY: float64(m.monitors[0].Y + m.monitors[0].Height),
	}

	for _, mon := range m.monitors[1:] {
		if float64(mon.X) < bounds.MinX {
			bounds.MinX = float64(mon.X)
		}
		if float64(mon.Y) < bounds.MinY {
			bounds.MinY = float64(mon.Y)
		}
		if float64(mon.X+mon.Width) > bounds.MaxX {
			bounds.MaxX = float64(mon.X + mon.Width)
		}
		if float64(mon.Y+mon.Height) > bounds.MaxY {
			bounds.MaxY = float64(mon.Y + mon.Height)
		}
	}

	return bounds
}

// drawMonitor draws a single monitor on the canvas.
func (m MonitorDisplay) drawMonitor(canvas [][]rune, mon domain.Monitor, bounds domain.DisplayBounds, scale float64, index int) {
	// Calculate scaled position
	x := int((float64(mon.X) - bounds.MinX) * scale)
	y := int((float64(mon.Y) - bounds.MinY) * scale)
	w := int(float64(mon.Width) * scale)
	h := int(float64(mon.Height) * scale)

	// Ensure minimum size
	if w < 3 {
		w = 3
	}
	if h < 2 {
		h = 2
	}

	// Clamp to canvas bounds
	maxY := len(canvas)
	maxX := 0
	if maxY > 0 {
		maxX = len(canvas[0])
	}

	// Choose border characters based on highlight state
	isHighlighted := mon.ID == m.highlightedID || mon.Name == m.highlightedID
	var hChar, vChar, tlChar, trChar, blChar, brChar rune
	switch {
	case isHighlighted:
		hChar, vChar = '=', '\u2551'
		tlChar, trChar, blChar, brChar = '\u2554', '\u2557', '\u255a', '\u255d'
	case mon.Primary:
		hChar, vChar = '\u2500', '\u2502'
		tlChar, trChar, blChar, brChar = '\u256d', '\u256e', '\u2570', '\u256f'
	default:
		hChar, vChar = '-', '|'
		tlChar, trChar, blChar, brChar = '+', '+', '+', '+'
	}

	// Draw top border
	if y >= 0 && y < maxY {
		for dx := 0; dx <= w && x+dx < maxX; dx++ {
			if x+dx >= 0 {
				switch dx {
				case 0:
					canvas[y][x+dx] = tlChar
				case w:
					canvas[y][x+dx] = trChar
				default:
					canvas[y][x+dx] = hChar
				}
			}
		}
	}

	// Draw bottom border
	bottomY := y + h
	if bottomY >= 0 && bottomY < maxY {
		for dx := 0; dx <= w && x+dx < maxX; dx++ {
			if x+dx >= 0 {
				switch dx {
				case 0:
					canvas[bottomY][x+dx] = blChar
				case w:
					canvas[bottomY][x+dx] = brChar
				default:
					canvas[bottomY][x+dx] = hChar
				}
			}
		}
	}

	// Draw side borders
	for dy := 1; dy < h; dy++ {
		if y+dy >= 0 && y+dy < maxY {
			if x >= 0 && x < maxX {
				canvas[y+dy][x] = vChar
			}
			if x+w >= 0 && x+w < maxX {
				canvas[y+dy][x+w] = vChar
			}
		}
	}

	// Draw label inside monitor
	if m.showLabels && h > 1 && w > 2 {
		labelY := y + h/2
		if labelY >= 0 && labelY < maxY {
			// Create label: number + primary indicator
			label := fmt.Sprintf("%d", index+1)
			if mon.Primary {
				label += "*"
			}

			// Center the label
			labelX := x + (w-len(label))/2
			for i, ch := range label {
				if labelX+i > x && labelX+i < x+w && labelX+i < maxX && labelX+i >= 0 {
					canvas[labelY][labelX+i] = ch
				}
			}
		}
	}
}

// drawCursor draws the cursor position on the canvas.
func (m MonitorDisplay) drawCursor(canvas [][]rune, bounds domain.DisplayBounds, scale float64) {
	if m.cursorPos == nil {
		return
	}

	x := int((m.cursorPos.X - bounds.MinX) * scale)
	y := int((m.cursorPos.Y - bounds.MinY) * scale)

	maxY := len(canvas)
	maxX := 0
	if maxY > 0 {
		maxX = len(canvas[0])
	}

	if y >= 0 && y < maxY && x >= 0 && x < maxX {
		canvas[y][x] = '\u2588' // Full block cursor
	}
}

// renderLegend renders the legend explaining the display.
func (m MonitorDisplay) renderLegend() string {
	var items []string

	// Monitor list with details
	for i, mon := range m.monitors {
		var entry string
		label := fmt.Sprintf("%d", i+1)
		if mon.Primary {
			label += "*"
		}

		name := mon.Name
		if name == "" {
			name = mon.ID
		}

		if m.showDimensions {
			entry = fmt.Sprintf("%s %s: %s (%dx%d)",
				styles.IconMonitor,
				styles.LabelStyle.Render(label),
				name,
				mon.Width,
				mon.Height,
			)
			if mon.RefreshRate > 0 {
				entry += fmt.Sprintf(" @%dHz", mon.RefreshRate)
			}
		} else {
			entry = fmt.Sprintf("%s %s: %s",
				styles.IconMonitor,
				styles.LabelStyle.Render(label),
				name,
			)
		}

		items = append(items, styles.MutedStyle.Render(entry))
	}

	// Primary indicator note
	hasPrimary := false
	for _, mon := range m.monitors {
		if mon.Primary {
			hasPrimary = true
			break
		}
	}
	if hasPrimary {
		items = append(items, styles.MutedStyle.Render("* = Primary monitor"))
	}

	return lipgloss.JoinVertical(lipgloss.Left, items...)
}

// CompactView renders a compact single-line representation.
func (m MonitorDisplay) CompactView() string {
	if len(m.monitors) == 0 {
		return styles.MutedStyle.Render("No monitors")
	}

	var parts []string
	for i, mon := range m.monitors {
		label := fmt.Sprintf("%d", i+1)
		if mon.Primary {
			label += "*"
		}
		parts = append(parts, fmt.Sprintf("%s:%dx%d", label, mon.Width, mon.Height))
	}

	return styles.IconMonitor + " " + strings.Join(parts, " | ")
}
