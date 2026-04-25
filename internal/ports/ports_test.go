package ports

import (
	"bytes"
	"net"
	"strings"
	"testing"
)

func TestIsPortInUse_Free(t *testing.T) {
	// A high random port should be free
	if IsPortInUse(0) {
		t.Error("port 0 (any) should not be considered in use")
	}
}

func TestIsPortInUse_Bound(t *testing.T) {
	// Bind a port and verify it's detected as in use
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind port: %v", err)
	}
	defer ln.Close()

	port := ln.Addr().(*net.TCPAddr).Port
	if !IsPortInUse(port) {
		t.Errorf("port %d should be detected as in use", port)
	}
}

func TestFindFreePort(t *testing.T) {
	port, err := FindFreePort(19000)
	if err != nil {
		t.Fatalf("FindFreePort() error: %v", err)
	}
	if port < 19000 || port >= 19100 {
		t.Errorf("port %d out of expected range [19000, 19100)", port)
	}
}

func TestFindFreePort_Conflict(t *testing.T) {
	// Bind a port and verify FindFreePort skips it
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to bind port: %v", err)
	}
	defer ln.Close()

	boundPort := ln.Addr().(*net.TCPAddr).Port
	found, err := FindFreePort(boundPort)
	if err != nil {
		t.Fatalf("FindFreePort() error: %v", err)
	}
	if found == boundPort {
		t.Errorf("FindFreePort should skip bound port %d", boundPort)
	}
}

func TestResolveAll(t *testing.T) {
	desired := map[string]int{"test-svc": 19500}
	resolved, remaps, err := ResolveAll(desired)
	if err != nil {
		t.Fatalf("ResolveAll() error: %v", err)
	}
	if len(remaps) != 0 {
		t.Error("expected no remaps for free port")
	}
	if resolved["test-svc"] != 19500 {
		t.Errorf("expected port 19500, got %d", resolved["test-svc"])
	}
}

func TestResolveAll_WithConflict(t *testing.T) {
	// Bind a port to force a remap
	ln, err := net.Listen("tcp", ":0")
	if err != nil {
		t.Fatalf("failed to bind port: %v", err)
	}
	defer ln.Close()

	boundPort := ln.Addr().(*net.TCPAddr).Port
	desired := map[string]int{"conflicted": boundPort}
	resolved, remaps, err := ResolveAll(desired)
	if err != nil {
		t.Fatalf("ResolveAll() error: %v", err)
	}
	if len(remaps) != 1 {
		t.Fatalf("expected 1 remap, got %d", len(remaps))
	}
	if remaps[0].From != boundPort {
		t.Errorf("remap From should be %d", boundPort)
	}
	if resolved["conflicted"] == boundPort {
		t.Error("resolved port should differ from bound port")
	}
}

func TestApplyRemaps(t *testing.T) {
	compose := `    ports:
      - "5432:5432"`
	env := "DATABASE_URL=postgresql://postgres:5432/mydb"

	remaps := []Remap{{Service: "postgres", From: 5432, To: 5433}}
	newCompose, newEnv := ApplyRemaps(compose, env, remaps)

	if !strings.Contains(newCompose, "5433:5432") {
		t.Error("compose should have remapped host port to 5433")
	}
	if !strings.Contains(newEnv, ":5433/") {
		t.Error("env should have remapped port to 5433")
	}
}

func TestPrintRemaps(t *testing.T) {
	var buf bytes.Buffer
	remaps := []Remap{{Service: "postgres", From: 5432, To: 5433}}
	PrintRemaps(remaps, &buf)
	output := buf.String()
	if !strings.Contains(output, "5432") || !strings.Contains(output, "5433") {
		t.Error("PrintRemaps should mention both old and new ports")
	}
}
