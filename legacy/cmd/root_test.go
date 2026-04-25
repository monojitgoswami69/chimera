package cmd

import (
	"testing"
)

func TestRootCommand(t *testing.T) {
	// Test that root command can be executed
	rootCmd.SetArgs([]string{"--help"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("Root command failed: %v", err)
	}
}

func TestVersionFlag(t *testing.T) {
	// Test version flag
	rootCmd.SetArgs([]string{"--version"})
	if err := rootCmd.Execute(); err != nil {
		t.Errorf("Version flag failed: %v", err)
	}
}

func TestGetVerbose(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "verbose flag set",
			args:     []string{"--verbose"},
			expected: true,
		},
		{
			name:     "verbose flag short form",
			args:     []string{"-v"},
			expected: true,
		},
		{
			name:     "verbose flag not set",
			args:     []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each subtest to avoid state leakage
			rootCmd.ResetFlags()
			rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
			rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-error output")

			rootCmd.SetArgs(tt.args)
			rootCmd.ParseFlags(tt.args)
			result := GetVerbose(rootCmd)
			if result != tt.expected {
				t.Errorf("GetVerbose() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestGetQuiet(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		expected bool
	}{
		{
			name:     "quiet flag set",
			args:     []string{"--quiet"},
			expected: true,
		},
		{
			name:     "quiet flag short form",
			args:     []string{"-q"},
			expected: true,
		},
		{
			name:     "quiet flag not set",
			args:     []string{},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset flags before each subtest to avoid state leakage
			rootCmd.ResetFlags()
			rootCmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
			rootCmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress non-error output")

			rootCmd.SetArgs(tt.args)
			rootCmd.ParseFlags(tt.args)
			result := GetQuiet(rootCmd)
			if result != tt.expected {
				t.Errorf("GetQuiet() = %v, want %v", result, tt.expected)
			}
		})
	}
}
