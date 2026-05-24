package ui

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// SpinnerFrames are the braille frames used for the spinner.
var SpinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// Spinner is an animated spinner that runs in its own goroutine.
type Spinner struct {
	message string
	stop    chan struct{}
	wg      sync.WaitGroup
	active  atomic.Bool
}

// StartSpinner begins an animated spinner with the given message. It is safe to
// call .Stop() multiple times; only the first call has an effect.
func StartSpinner(message string) *Spinner {
	s := &Spinner{message: message, stop: make(chan struct{})}
	s.active.Store(true)
	s.wg.Add(1)
	go s.run()
	return s
}

func (s *Spinner) run() {
	defer s.wg.Done()
	t := time.NewTicker(90 * time.Millisecond)
	defer t.Stop()
	i := 0
	for {
		select {
		case <-s.stop:
			// Erase line on stop.
			fmt.Print("\r\033[2K")
			return
		case <-t.C:
			frame := SpinnerFrames[i%len(SpinnerFrames)]
			fmt.Printf("\r  %s  %s", PrimaryStyle.Render(frame), s.message)
			i++
		}
	}
}

// Stop halts the spinner and clears the line.
func (s *Spinner) Stop() {
	if s.active.CompareAndSwap(true, false) {
		close(s.stop)
		s.wg.Wait()
	}
}

// Success stops the spinner and prints a success line in its place.
func (s *Spinner) Success(msg string) {
	s.Stop()
	fmt.Println(SuccessLine(msg))
}

// Fail stops the spinner and prints an error line in its place.
func (s *Spinner) Fail(msg string) {
	s.Stop()
	fmt.Println(ErrorLine(msg))
}

// SpinnerLine formats a static spinner line (no animation).
func SpinnerLine(frame, message string) string {
	return fmt.Sprintf("  %s  %s", PrimaryStyle.Render(frame), message)
}

// SuccessLine formats a success line.
func SuccessLine(message string) string {
	return fmt.Sprintf("  %s  %s", SuccessStyle.Render("✓"), message)
}

// ErrorLine formats an error line.
func ErrorLine(message string) string {
	return fmt.Sprintf("  %s  %s", ErrorStyle.Render("✗"), message)
}

// WarningLine formats a warning line.
func WarningLine(message string) string {
	return fmt.Sprintf("  %s  %s", WarningStyle.Render("⚠"), message)
}

// InfoLine formats an info line.
func InfoLine(message string) string {
	return fmt.Sprintf("  %s  %s", PrimaryStyle.Render("ℹ"), message)
}

// IndentLine formats an indented continuation line.
func IndentLine(message string) string {
	return fmt.Sprintf("  ↳  %s", MutedStyle.Render(message))
}
