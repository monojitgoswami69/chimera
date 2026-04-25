package cmd

import "testing"

func TestIsProjectImageTag(t *testing.T) {
	tests := []struct {
		name    string
		tag     string
		project string
		want    bool
	}{
		{name: "exact project tag", tag: "myapp:latest", project: "myapp", want: true},
		{name: "docker compose hyphen name", tag: "myapp-app:latest", project: "myapp", want: true},
		{name: "docker compose underscore name", tag: "myapp_app:latest", project: "myapp", want: true},
		{name: "registry prefixed", tag: "ghcr.io/acme/myapp-app:latest", project: "myapp", want: true},
		{name: "different project", tag: "other-app:latest", project: "myapp", want: false},
		{name: "dangling image", tag: "<none>:<none>", project: "myapp", want: false},
		{name: "empty tag", tag: "", project: "myapp", want: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isProjectImageTag(tc.tag, tc.project)
			if got != tc.want {
				t.Fatalf("isProjectImageTag(%q, %q) = %v, want %v", tc.tag, tc.project, got, tc.want)
			}
		})
	}
}
