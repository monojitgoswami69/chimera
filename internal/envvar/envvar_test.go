package envvar

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"chimera/internal/detector"
)

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestNextPublicCapturesFullName(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "page.tsx"), `
		const url = process.env.NEXT_PUBLIC_API_URL;
		const x = process.env.NEXT_PUBLIC_FOO;
	`)
	svcs := []detector.Service{{ID: "service-1", Directory: "", Language: "javascript", Framework: "next", Type: "frontend"}}
	got := Detect(root, svcs)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	names := map[string]bool{}
	for _, v := range got[0].Vars {
		names[v.Name] = true
	}
	if !names["NEXT_PUBLIC_API_URL"] {
		t.Errorf("did not capture NEXT_PUBLIC_API_URL; got: %v", names)
	}
	if !names["NEXT_PUBLIC_FOO"] {
		t.Errorf("did not capture NEXT_PUBLIC_FOO; got: %v", names)
	}
	if names["API_URL"] {
		t.Errorf("captured stripped suffix API_URL, want full name")
	}
}

func TestPythonOsEnviron(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "main.py"), `
import os
DB = os.environ["DATABASE_URL"]
SECRET = os.getenv("SECRET_KEY")
PORT = os.environ.get("PORT")
`)
	svcs := []detector.Service{{ID: "service-1", Directory: "", Language: "python", Framework: "fastapi", Type: "backend"}}
	got := Detect(root, svcs)
	if len(got) != 1 {
		t.Fatalf("expected 1 result, got %d", len(got))
	}
	names := map[string]bool{}
	for _, v := range got[0].Vars {
		names[v.Name] = true
	}
	for _, want := range []string{"DATABASE_URL", "SECRET_KEY", "PORT"} {
		if !names[want] {
			t.Errorf("missing %s in detected vars: %v", want, names)
		}
	}
}

func TestDeterministicOrdering(t *testing.T) {
	root := t.TempDir()
	writeFile(t, filepath.Join(root, "a.js"), `
		process.env.ZED;
		process.env.ALPHA;
		process.env.MIKE;
	`)
	svcs := []detector.Service{{ID: "service-1", Language: "javascript", Framework: "node"}}
	first := Detect(root, svcs)[0].Vars

	// Re-run and confirm the order is identical across calls.
	second := Detect(root, svcs)[0].Vars
	if len(first) != len(second) {
		t.Fatalf("len mismatch: %d vs %d", len(first), len(second))
	}
	for i := range first {
		if first[i].Name != second[i].Name {
			t.Errorf("nondeterministic ordering at %d: %s vs %s", i, first[i].Name, second[i].Name)
		}
	}
	if first[0].Name != "ALPHA" || first[len(first)-1].Name != "ZED" {
		t.Errorf("not sorted: %v", first)
	}
}

func TestCommonVarsDoNotDuplicate(t *testing.T) {
	out := GenerateEnvExample([]EnvVar{{Name: "DATABASE_URL"}}, "fastapi")
	if c := strings.Count(out, "DATABASE_URL="); c != 1 {
		t.Errorf("DATABASE_URL appeared %d times in:\n%s", c, out)
	}
}
