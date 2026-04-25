package ui

import (
	"fmt"
	"time"
)

// Spinner frames (braille)
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// SpinnerState tracks spinner animation
type SpinnerState struct {
	frame     int
	startTime time.Time
}

// NewSpinner creates a new spinner
func NewSpinner() *SpinnerState {
	return &SpinnerState{
		frame:     0,
		startTime: time.Now(),
	}
}

// Next returns the next frame
func (s *SpinnerState) Next() string {
	frame := SpinnerFrames[s.frame%len(SpinnerFrames)]
	s.frame++
	return frame
}

// Elapsed returns elapsed time
func (s *SpinnerState) Elapsed() string {
	elapsed := time.Since(s.startTime).Seconds()
	return fmt.Sprintf("%.1fs", elapsed)
}

// SpinnerLine formats a spinner line
func SpinnerLine(frame, message string) string {
	return fmt.Sprintf("  %s  %s", PrimaryStyle.Render(frame), message)
}

// SuccessLine formats a success line
func SuccessLine(message string) string {
	return fmt.Sprintf("  %s  %s", SuccessStyle.Render("✓"), message)
}

// ErrorLine formats an error line
func ErrorLine(message string) string {
	return fmt.Sprintf("  %s  %s", ErrorStyle.Render("✗"), message)
}

// WarningLine formats a warning line
func WarningLine(message string) string {
	return fmt.Sprintf("  %s  %s", WarningStyle.Render("⚠"), message)
}

// InfoLine formats an info line
func InfoLine(message string) string {
	return fmt.Sprintf("  %s  %s", PrimaryStyle.Render("ℹ"), message)
}

// IndentLine formats an indented line
func IndentLine(message string) string {
	return fmt.Sprintf("  ↳  %s", MutedStyle.Render(message))
}
