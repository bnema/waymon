package ui

import (
	"strings"
	"testing"
)

func TestFormatControl(t *testing.T) {
	tests := []struct {
		name string
		key  string
		desc string
		want string
	}{
		{
			name: "basic control",
			key:  "q",
			desc: "Quit",
			want: "q - Quit",
		},
		{
			name: "longer key",
			key:  "Space",
			desc: "Toggle capture",
			want: "Space - Toggle capture",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatControl(tt.key, tt.desc)
			// Check that it contains both key and description
			if !strings.Contains(got, tt.key) {
				t.Errorf("FormatControl() missing key %q", tt.key)
			}
			if !strings.Contains(got, tt.desc) {
				t.Errorf("FormatControl() missing description %q", tt.desc)
			}
		})
	}
}

func TestFormatStatus(t *testing.T) {
	tests := []struct {
		name      string
		connected bool
		status    string
	}{
		{
			name:      "connected status",
			connected: true,
			status:    "Connected to server",
		},
		{
			name:      "disconnected status",
			connected: false,
			status:    "Disconnected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatStatus(tt.connected, tt.status)

			// Should contain the status text
			if !strings.Contains(got, tt.status) {
				t.Errorf("FormatStatus() missing status text %q", tt.status)
			}

			// Should have different indicators
			if tt.connected && !strings.Contains(got, "●") {
				t.Errorf("FormatStatus() connected=true should contain filled circle")
			}
			if !tt.connected && !strings.Contains(got, "○") {
				t.Errorf("FormatStatus() connected=false should contain empty circle")
			}
		})
	}
}

func TestFormatListItem(t *testing.T) {
	tests := []struct {
		name   string
		item   string
		active bool
	}{
		{
			name:   "inactive item",
			item:   "Server 1",
			active: false,
		},
		{
			name:   "active item",
			item:   "Server 2",
			active: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FormatListItem(tt.item, tt.active)

			// Should contain bullet point and item
			if !strings.Contains(got, "•") {
				t.Errorf("FormatListItem() missing bullet point")
			}
			if !strings.Contains(got, tt.item) {
				t.Errorf("FormatListItem() missing item text %q", tt.item)
			}
		})
	}
}

func TestCenter(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		content string
	}{
		{
			name:    "short content",
			width:   20,
			content: "Test",
		},
		{
			name:    "exact width",
			width:   4,
			content: "Test",
		},
		{
			name:    "content longer than width",
			width:   2,
			content: "Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Center(tt.width, tt.content)

			// Should contain the content
			if !strings.Contains(got, tt.content) {
				t.Errorf("Center() missing content %q", tt.content)
			}
		})
	}
}

func TestRight(t *testing.T) {
	tests := []struct {
		name    string
		width   int
		content string
	}{
		{
			name:    "short content",
			width:   20,
			content: "Test",
		},
		{
			name:    "exact width",
			width:   4,
			content: "Test",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Right(tt.width, tt.content)

			// Should contain the content
			if !strings.Contains(got, tt.content) {
				t.Errorf("Right() missing content %q", tt.content)
			}
		})
	}
}
