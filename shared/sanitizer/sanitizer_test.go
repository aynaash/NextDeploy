package sanitizer

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestStringSanitizers(t *testing.T) {
	tests := []struct {
		name string
		fn   func(string) string
		in   string
		want string
	}{
		{"DockerImageName valid", DockerImageName, "ghcr.io/org/app:v1.2", "ghcr.io/org/app:v1.2"},
		{"DockerImageName strips shell chars", DockerImageName, "app;rm -rf /", "apprm-rf/"},
		{"DockerImageName empty", DockerImageName, "   ", ""},
		{"DockerImageName too long", DockerImageName, strings.Repeat("a", 256), ""},
		{"ContainerName valid", ContainerName, "my_app-1.0", "my_app-1.0"},
		{"ContainerName leading dash", ContainerName, "-app$1", "c-app1"},
		{"ContainerName truncates to 64", ContainerName, strings.Repeat("a", 80), strings.Repeat("a", 64)},
		{"CommandArgument strips metachars", CommandArgument, "hello;world$(id)\n", "helloworldid"},
		{"Password strips metachars", Password, "p@ss;word$`\n", "p@ssword"},
		{"Alphanumeric", Alphanumeric, "a1-b2_c3!", "a1b2c3"},
		{"Filename traversal", Filename, "../../etc/passwd", "__etc_passwd"},
		{"Filename empty fallback", Filename, "", "file"},
		{"ShellCommand collapses spaces", ShellCommand, "ls  -la; /tmp", "ls -la /tmp"},
		{"URL https", URL, "https://example.com/path?a=1", "https://example.com/path?a=1"},
		{"URL rejects non-http", URL, "javascript:alert(1)", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.fn(tt.in); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestFilePath(t *testing.T) {
	base := t.TempDir()
	inside := filepath.Join(base, "sub", "file.txt")

	got, err := FilePath(inside, base)
	if err != nil {
		t.Fatalf("FilePath valid: %v", err)
	}
	if !strings.HasPrefix(got, base) {
		t.Fatalf("got %q, want prefix %q", got, base)
	}

	traversal := filepath.Join(base, "..", "outside.txt")
	_, err = FilePath(traversal, base)
	if err == nil {
		t.Fatal("expected security error for path traversal")
	}
	if !IsSecurityError(err) {
		t.Fatalf("expected IsSecurityError(err) to be true, got %T", err)
	}
}

func TestSafeExecArgs(t *testing.T) {
	cmd, args, err := SafeExecArgs("git-1", []string{"status", "--short;rm"})
	if err != nil {
		t.Fatalf("SafeExecArgs valid: %v", err)
	}
	if cmd != "git1" || len(args) != 2 || args[1] != "--shortrm" {
		t.Fatalf("unexpected output: cmd=%q args=%v", cmd, args)
	}

	if _, _, err := SafeExecArgs(";;;", nil); !IsSecurityError(err) {
		t.Fatalf("expected SecurityError for empty command, got %v", err)
	}
	if _, _, err := SafeExecArgs("echo", []string{";;;"}); !IsSecurityError(err) {
		t.Fatalf("expected SecurityError for fully stripped arg, got %v", err)
	}
}
