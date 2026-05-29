package store

import (
	"path/filepath"
	"testing"
)

// envMap builds an env lookup func over a fixed map for table tests.
func envMap(m map[string]string) func(string) string {
	return func(k string) string { return m[k] }
}

func TestStorageRoot(t *testing.T) {
	const id = "abc123"

	tests := []struct {
		name string
		home string
		goos string
		env  map[string]string
		want string
	}{
		{
			name: "darwin default (A8)",
			home: "/Users/dev",
			goos: "darwin",
			env:  nil,
			want: filepath.Join("/Users/dev", "Library", "Application Support", "limbo", "projects", id),
		},
		{
			name: "linux default, XDG unset (A9)",
			home: "/home/dev",
			goos: "linux",
			env:  nil,
			want: filepath.Join("/home/dev", ".local", "share", "limbo", "projects", id),
		},
		{
			name: "linux default, XDG empty falls back to ~/.local/share (A9)",
			home: "/home/dev",
			goos: "linux",
			env:  map[string]string{"XDG_DATA_HOME": ""},
			want: filepath.Join("/home/dev", ".local", "share", "limbo", "projects", id),
		},
		{
			name: "linux honors non-empty XDG_DATA_HOME (A9)",
			home: "/home/dev",
			goos: "linux",
			env:  map[string]string{"XDG_DATA_HOME": "/xdg/data"},
			want: filepath.Join("/xdg/data", "limbo", "projects", id),
		},
		{
			name: "LIMBO_HOME overrides darwin default (A10)",
			home: "/Users/dev",
			goos: "darwin",
			env:  map[string]string{"LIMBO_HOME": "/custom/limbo"},
			want: filepath.Join("/custom/limbo", "projects", id),
		},
		{
			name: "LIMBO_HOME overrides linux + XDG (A10)",
			home: "/home/dev",
			goos: "linux",
			env: map[string]string{
				"LIMBO_HOME":    "/custom/limbo",
				"XDG_DATA_HOME": "/xdg/data",
			},
			want: filepath.Join("/custom/limbo", "projects", id),
		},
		{
			name: "empty LIMBO_HOME does not override (A10)",
			home: "/Users/dev",
			goos: "darwin",
			env:  map[string]string{"LIMBO_HOME": ""},
			want: filepath.Join("/Users/dev", "Library", "Application Support", "limbo", "projects", id),
		},
		{
			name: "unknown goos treated as linux/unix (A9)",
			home: "/home/dev",
			goos: "freebsd",
			env:  nil,
			want: filepath.Join("/home/dev", ".local", "share", "limbo", "projects", id),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := StorageRoot(tt.home, tt.goos, envMap(tt.env), id)
			if got != tt.want {
				t.Fatalf("StorageRoot(%q, %q, env, %q) = %q, want %q",
					tt.home, tt.goos, id, got, tt.want)
			}
		})
	}
}

func TestStorageRootNilEnv(t *testing.T) {
	// A nil env lookup must be treated as "everything unset" and not panic.
	got := StorageRoot("/home/dev", "linux", nil, "id")
	want := filepath.Join("/home/dev", ".local", "share", "limbo", "projects", "id")
	if got != want {
		t.Fatalf("StorageRoot with nil env = %q, want %q", got, want)
	}
}
