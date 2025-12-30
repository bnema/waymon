// Package styles provides global styling for the TUI.
package styles

// Nerd Font icons using unicode escapes.
// Reference: https://www.nerdfonts.com/cheat-sheet
const (
	// Status icons
	IconCheck       = "\uf00c" //  (fa-check)
	IconCross       = "\uf00d" //  (fa-times)
	IconWarning     = "\uf071" //  (fa-exclamation-triangle)
	IconInfo        = "\uf05a" //  (fa-info-circle)
	IconQuestion    = "\uf059" //  (fa-question-circle)
	IconSpinner     = "\uf110" //  (fa-spinner)
	IconCircle      = "\uf111" //  (fa-circle)
	IconCircleEmpty = "\uf10c" //  (fa-circle-o)

	// Device icons
	IconServer   = "\uf233" //  (fa-server)
	IconClient   = "\uf108" //  (fa-desktop)
	IconLaptop   = "\uf109" //  (fa-laptop)
	IconMonitor  = "\uf26c" //  (fa-tv)
	IconMouse    = "\uf87c" // 󰡜 (md-mouse)
	IconKeyboard = "\uf11c" //  (fa-keyboard-o)
	IconDisplay  = "\uf878" // 󰡸 (md-monitor)

	// Connection icons
	IconConnected    = "\uf1e6" //  (fa-plug)
	IconDisconnected = "\uf127" //  (fa-chain-broken)
	IconNetwork      = "\uf6ff" // 󰛿 (md-lan)
	IconWifi         = "\uf1eb" //  (fa-wifi)
	IconSSH          = "\ue795" //  (dev-terminal)

	// Navigation/Direction icons
	IconArrowRight = "\uf061" //  (fa-arrow-right)
	IconArrowLeft  = "\uf060" //  (fa-arrow-left)
	IconArrowUp    = "\uf062" //  (fa-arrow-up)
	IconArrowDown  = "\uf063" //  (fa-arrow-down)
	IconChevronR   = "\uf054" //  (fa-chevron-right)
	IconChevronL   = "\uf053" //  (fa-chevron-left)
	IconChevronU   = "\uf077" //  (fa-chevron-up)
	IconChevronD   = "\uf078" //  (fa-chevron-down)

	// Action icons
	IconSwitch   = "\uf021" //  (fa-refresh)
	IconRelease  = "\uf09c" //  (fa-unlock)
	IconLock     = "\uf023" //  (fa-lock)
	IconQuit     = "\uf011" //  (fa-power-off)
	IconSettings = "\uf013" //  (fa-cog)
	IconHelp     = "\uf128" //  (fa-question)
	IconClose    = "\uf00d" //  (fa-times)

	// Edge/Position icons (for monitor edge mapping)
	IconEdgeTop    = "\uf077" //  (fa-chevron-up)
	IconEdgeBottom = "\uf078" //  (fa-chevron-down)
	IconEdgeLeft   = "\uf053" //  (fa-chevron-left)
	IconEdgeRight  = "\uf054" //  (fa-chevron-right)

	// Misc icons
	IconDot       = "\uf444" //  (oct-dot-fill)
	IconBullet    = "\u2022" // •
	IconStar      = "\uf005" //  (fa-star)
	IconHeart     = "\uf004" //  (fa-heart)
	IconClock     = "\uf017" //  (fa-clock-o)
	IconCalendar  = "\uf073" //  (fa-calendar)
	IconFolder    = "\uf07b" //  (fa-folder)
	IconFile      = "\uf15b" //  (fa-file)
	IconTerminal  = "\uf120" //  (fa-terminal)
	IconCode      = "\uf121" //  (fa-code)
	IconGear      = "\uf013" //  (fa-cog)
	IconUser      = "\uf007" //  (fa-user)
	IconUsers     = "\uf0c0" //  (fa-users)
	IconHome      = "\uf015" //  (fa-home)
	IconPin       = "\uf08d" //  (fa-thumb-tack)
	IconFlag      = "\uf024" //  (fa-flag)
	IconBell      = "\uf0f3" //  (fa-bell)
	IconBellSlash = "\uf1f6" //  (fa-bell-slash)
)

// StatusIcon returns the appropriate icon for a connection status.
func StatusIcon(connected bool) string {
	if connected {
		return IconCheck
	}
	return IconCross
}

// BoolIcon returns a check or cross icon based on the boolean value.
func BoolIcon(b bool) string {
	if b {
		return IconCheck
	}
	return IconCross
}
