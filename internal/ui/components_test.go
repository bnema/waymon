package ui

import (
	"strings"
	"testing"
)

func TestStatusBar(t *testing.T) {
	t.Run("creates new status bar", func(t *testing.T) {
		sb := NewStatusBar("Test App")
		
		if sb.Title != "Test App" {
			t.Errorf("Expected title 'Test App', got %q", sb.Title)
		}
		if !sb.ShowSpinner {
			t.Error("Expected ShowSpinner to be true by default")
		}
	})

	t.Run("renders status bar", func(t *testing.T) {
		sb := NewStatusBar("Test App")
		sb.Width = 80
		sb.Status = "Running"
		sb.Connected = true
		
		view := sb.View()
		
		if !strings.Contains(view, "Test App") {
			t.Error("Status bar should contain title")
		}
		if !strings.Contains(view, "Running") {
			t.Error("Status bar should contain status")
		}
	})
}

func TestInfoPanel(t *testing.T) {
	tests := []struct {
		name    string
		panel   InfoPanel
		mustHave []string
	}{
		{
			name: "with title",
			panel: InfoPanel{
				Title:   "Server Info",
				Content: []string{"Line 1", "Line 2"},
				Width:   50,
			},
			mustHave: []string{"Server Info", "Line 1", "Line 2"},
		},
		{
			name: "without title",
			panel: InfoPanel{
				Content: []string{"Just content"},
				Width:   50,
			},
			mustHave: []string{"Just content"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.panel.View()
			
			for _, must := range tt.mustHave {
				if !strings.Contains(view, must) {
					t.Errorf("InfoPanel should contain %q", must)
				}
			}
		})
	}
}

func TestConnectionList(t *testing.T) {
	t.Run("empty list", func(t *testing.T) {
		cl := ConnectionList{
			Title:       "Connections",
			Connections: []Connection{},
			Width:       60,
		}
		
		view := cl.View()
		
		if !strings.Contains(view, "Connections") {
			t.Error("Should contain title")
		}
		if !strings.Contains(view, "No connections") {
			t.Error("Should show 'No connections' when empty")
		}
	})

	t.Run("with connections", func(t *testing.T) {
		cl := ConnectionList{
			Title: "Connections",
			Connections: []Connection{
				{Name: "Server1", Address: "192.168.1.1", Connected: true},
				{Name: "Server2", Address: "192.168.1.2", Connected: false},
			},
			Width: 60,
		}
		
		view := cl.View()
		
		if !strings.Contains(view, "Server1") {
			t.Error("Should contain Server1")
		}
		if !strings.Contains(view, "Server2") {
			t.Error("Should contain Server2")
		}
		if !strings.Contains(view, "192.168.1.1") {
			t.Error("Should contain addresses")
		}
	})
}

func TestMonitorInfo(t *testing.T) {
	mi := MonitorInfo{
		Monitors: []Monitor{
			{
				Name:     "DP-1",
				Size:     "1920x1080",
				Position: "0,0",
				Primary:  true,
			},
			{
				Name:     "DP-2",
				Size:     "1920x1080",
				Position: "1920,0",
				Primary:  false,
			},
		},
		Width: 80,
	}
	
	view := mi.View()
	
	if !strings.Contains(view, "Detected 2 monitor(s)") {
		t.Error("Should show monitor count")
	}
	if !strings.Contains(view, "DP-1") {
		t.Error("Should contain DP-1")
	}
	if !strings.Contains(view, "DP-2") {
		t.Error("Should contain DP-2")
	}
	if !strings.Contains(view, "(primary)") {
		t.Error("Should indicate primary monitor")
	}
	if !strings.Contains(view, "1920x1080") {
		t.Error("Should show monitor size")
	}
}

func TestControlsHelp(t *testing.T) {
	ch := ControlsHelp{
		Controls: []Control{
			{Key: "q", Desc: "Quit"},
			{Key: "Space", Desc: "Toggle capture"},
			{Key: "r", Desc: "Reconnect"},
		},
		Width: 60,
	}
	
	view := ch.View()
	
	if !strings.Contains(view, "Controls:") {
		t.Error("Should have Controls header")
	}
	
	for _, ctrl := range ch.Controls {
		if !strings.Contains(view, ctrl.Key) {
			t.Errorf("Should contain key %q", ctrl.Key)
		}
		if !strings.Contains(view, ctrl.Desc) {
			t.Errorf("Should contain description %q", ctrl.Desc)
		}
	}
}

func TestProgressIndicator(t *testing.T) {
	tests := []struct {
		name     string
		progress ProgressIndicator
		mustHave []string
	}{
		{
			name: "half progress",
			progress: ProgressIndicator{
				Label:          "Loading",
				Current:        50,
				Total:          100,
				Width:          40,
				ShowPercentage: true,
			},
			mustHave: []string{"Loading", "50%"},
		},
		{
			name: "without percentage",
			progress: ProgressIndicator{
				Label:          "Processing",
				Current:        25,
				Total:          100,
				Width:          40,
				ShowPercentage: false,
			},
			mustHave: []string{"Processing"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.progress.View()
			
			for _, must := range tt.mustHave {
				if !strings.Contains(view, must) {
					t.Errorf("Progress indicator should contain %q", must)
				}
			}
		})
	}
}

func TestMessage(t *testing.T) {
	tests := []struct {
		name        string
		msg         Message
		mustContain string
		prefix      string
	}{
		{
			name:        "info message",
			msg:         Message{Type: MessageInfo, Content: "Information"},
			mustContain: "Information",
			prefix:      "ℹ",
		},
		{
			name:        "success message",
			msg:         Message{Type: MessageSuccess, Content: "Success!"},
			mustContain: "Success!",
			prefix:      "✓",
		},
		{
			name:        "warning message",
			msg:         Message{Type: MessageWarning, Content: "Warning!"},
			mustContain: "Warning!",
			prefix:      "⚠",
		},
		{
			name:        "error message",
			msg:         Message{Type: MessageError, Content: "Error!"},
			mustContain: "Error!",
			prefix:      "✗",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			view := tt.msg.View()
			
			if !strings.Contains(view, tt.mustContain) {
				t.Errorf("Message should contain %q", tt.mustContain)
			}
			if !strings.Contains(view, tt.prefix) {
				t.Errorf("Message should have prefix %q", tt.prefix)
			}
		})
	}
}