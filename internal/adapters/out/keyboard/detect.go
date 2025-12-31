package keyboard

import (
	"context"
	"os"
	"os/exec"
	"strings"

	"github.com/bnema/waymon/internal/domain"
	"github.com/rs/zerolog"
)

// DetectSystemLayout detects the system's current keyboard layout.
// It tries multiple detection methods in order of preference:
// 1. XKB_DEFAULT_LAYOUT environment variable
// 2. localectl command
// 3. /etc/default/keyboard file
// Returns LayoutUS as default if detection fails.
func DetectSystemLayout(ctx context.Context) domain.KeyboardLayout {
	log := zerolog.Ctx(ctx)

	// Try XKB_DEFAULT_LAYOUT env var first
	if layout := os.Getenv("XKB_DEFAULT_LAYOUT"); layout != "" {
		normalized := normalizeLayoutName(layout)
		if normalized.IsValid() {
			log.Debug().Str("layout", string(normalized)).Msg("detected keyboard layout from XKB_DEFAULT_LAYOUT")
			return normalized
		}
	}

	// Try localectl
	if layout := detectFromLocalectl(ctx); layout.IsValid() {
		log.Debug().Str("layout", string(layout)).Msg("detected keyboard layout from localectl")
		return layout
	}

	// Try /etc/default/keyboard
	if layout := detectFromDefaultKeyboard(ctx); layout.IsValid() {
		log.Debug().Str("layout", string(layout)).Msg("detected keyboard layout from /etc/default/keyboard")
		return layout
	}

	// Default to US
	log.Debug().Msg("could not detect keyboard layout, defaulting to US")
	return domain.LayoutUS
}

// detectFromLocalectl tries to get the layout from localectl status.
func detectFromLocalectl(ctx context.Context) domain.KeyboardLayout {
	log := zerolog.Ctx(ctx)

	cmd := exec.CommandContext(ctx, "localectl", "status")
	output, err := cmd.Output()
	if err != nil {
		log.Debug().Err(err).Msg("localectl command failed")
		return ""
	}

	// Parse output looking for "X11 Layout:" line
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "X11 Layout:") {
			layout := strings.TrimSpace(strings.TrimPrefix(line, "X11 Layout:"))
			// Handle multiple layouts (e.g., "us,fr") - take the first one
			if idx := strings.Index(layout, ","); idx > 0 {
				layout = layout[:idx]
			}
			return normalizeLayoutName(layout)
		}
		// Also check VC Keymap for console layout
		if strings.HasPrefix(line, "VC Keymap:") {
			layout := strings.TrimSpace(strings.TrimPrefix(line, "VC Keymap:"))
			return normalizeLayoutName(layout)
		}
	}

	return ""
}

// detectFromDefaultKeyboard tries to read /etc/default/keyboard.
func detectFromDefaultKeyboard(ctx context.Context) domain.KeyboardLayout {
	log := zerolog.Ctx(ctx)

	data, err := os.ReadFile("/etc/default/keyboard")
	if err != nil {
		log.Debug().Err(err).Msg("could not read /etc/default/keyboard")
		return ""
	}

	// Parse looking for XKBLAYOUT="..."
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "XKBLAYOUT=") {
			layout := strings.TrimPrefix(line, "XKBLAYOUT=")
			layout = strings.Trim(layout, `"'`)
			// Handle multiple layouts
			if idx := strings.Index(layout, ","); idx > 0 {
				layout = layout[:idx]
			}
			return normalizeLayoutName(layout)
		}
	}

	return ""
}

// normalizeLayoutName converts various layout name formats to our standard format.
func normalizeLayoutName(name string) domain.KeyboardLayout {
	name = strings.ToLower(strings.TrimSpace(name))

	// Map common variations to our standard names
	switch name {
	case "us", "en_us", "us-intl", "us-alt-intl":
		return domain.LayoutUS
	case "fr", "fr-latin9", "french", "azerty":
		return domain.LayoutFR
	case "de", "german", "de-latin1", "qwertz":
		return domain.LayoutDE
	case "es", "spanish", "es-latin1":
		return domain.LayoutES
	case "uk", "gb", "en_gb", "british":
		return domain.LayoutUK
	case "it", "italian", "it-latin1":
		return domain.LayoutIT
	default:
		// Try direct match
		layout := domain.KeyboardLayout(name)
		if layout.IsValid() {
			return layout
		}
		return ""
	}
}
