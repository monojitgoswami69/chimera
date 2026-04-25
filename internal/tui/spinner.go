package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Spinner represents a TUI spinner for long-running operations
type Spinner struct {
	message string
	program *tea.Program
	done    chan struct{}
}

// spinnerModel is the bubbletea model for the spinner
type spinnerModel struct {
	message string
	frame   int
	done    bool
	result  string
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type tickMsg time.Time
type doneMsg struct {
	result string
}

// NewSpinner creates a new spinner with the given message
func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		done:    make(chan struct{}),
	}
}

// Start begins the spinner animation
func (s *Spinner) Start() error {
	model := spinnerModel{
		message: s.message,
		frame:   0,
		done:    false,
	}

	s.program = tea.NewProgram(model)
	
	go func() {
		if _, err := s.program.Run(); err != nil {
			// Silently handle errors to avoid breaking the terminal
		}
	}()

	// Give the program time to start
	time.Sleep(10 * time.Millisecond)
	
	return nil
}

// Update changes the spinner message
func (s *Spinner) Update(message string) {
	s.message = message
	if s.program != nil {
		s.program.Send(updateMsg{message: message})
	}
}

// Success stops the spinner with a success message
func (s *Spinner) Success(message string) {
	if s.program != nil {
		s.program.Send(doneMsg{result: SuccessStyle.Render("✓ " + message)})
		time.Sleep(50 * time.Millisecond)
	}
}

// Error stops the spinner with an error message
func (s *Spinner) Error(message string) {
	if s.program != nil {
		s.program.Send(doneMsg{result: ErrorStyle.Render("✗ " + message)})
		time.Sleep(50 * time.Millisecond)
	}
}

// Warning stops the spinner with a warning message
func (s *Spinner) Warning(message string) {
	if s.program != nil {
		s.program.Send(doneMsg{result: WarningStyle.Render("⚠ " + message)})
		time.Sleep(50 * time.Millisecond)
	}
}

// Info stops the spinner with an info message
func (s *Spinner) Info(message string) {
	if s.program != nil {
		s.program.Send(doneMsg{result: InfoStyle.Render("ℹ " + message)})
		time.Sleep(50 * time.Millisecond)
	}
}

// Stop stops the spinner
func (s *Spinner) Stop() {
	if s.program != nil {
		s.program.Quit()
		s.program = nil
	}
}

type updateMsg struct {
	message string
}

// Init initializes the spinner model
func (m spinnerModel) Init() tea.Cmd {
	return tick()
}

// Update handles messages for the spinner model
func (m spinnerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" {
			return m, tea.Quit
		}
	case tickMsg:
		if !m.done {
			m.frame = (m.frame + 1) % len(spinnerFrames)
			return m, tick()
		}
	case updateMsg:
		m.message = msg.message
		return m, nil
	case doneMsg:
		m.done = true
		m.result = msg.result
		return m, tea.Quit
	}
	return m, nil
}

// View renders the spinner
func (m spinnerModel) View() string {
	if m.done {
		return m.result + "\n"
	}
	
	spinner := lipgloss.NewStyle().
		Foreground(lipgloss.Color("205")).
		Render(spinnerFrames[m.frame])
	
	return fmt.Sprintf("%s %s", spinner, m.message)
}

// tick returns a command that sends a tick message
func tick() tea.Cmd {
	return tea.Tick(100*time.Millisecond, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}
