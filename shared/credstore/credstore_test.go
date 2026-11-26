package credstore

import (
	"errors"
	"reflect"
	"sort"
	"testing"
)

func TestCredstore_RoundTripAndDelete(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	if _, err := Load("vps"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load(missing) = %v, want ErrNotFound", err)
	}

	vpsFields := map[string]string{
		"host":     "203.0.113.10",
		"username": "ubuntu",
		"key_path": "~/.ssh/id_ed25519",
	}
	if err := Save("vps", vpsFields); err != nil {
		t.Fatalf("Save(vps): %v", err)
	}

	stagingFields := map[string]string{
		"host": "198.51.100.20",
	}
	if err := Save("vps-staging", stagingFields); err != nil {
		t.Fatalf("Save(vps-staging): %v", err)
	}

	gotVPS, err := Load("vps")
	if err != nil {
		t.Fatalf("Load(vps): %v", err)
	}
	if !reflect.DeepEqual(gotVPS, vpsFields) {
		t.Fatalf("Load(vps) = %v, want %v", gotVPS, vpsFields)
	}

	gotStaging, err := Load("vps-staging")
	if err != nil {
		t.Fatalf("Load(vps-staging): %v", err)
	}
	if !reflect.DeepEqual(gotStaging, stagingFields) {
		t.Fatalf("Load(vps-staging) = %v, want %v", gotStaging, stagingFields)
	}

	providers, err := List()
	if err != nil {
		t.Fatalf("List(): %v", err)
	}
	sort.Strings(providers)
	wantProviders := []string{"vps", "vps-staging"}
	if !reflect.DeepEqual(providers, wantProviders) {
		t.Fatalf("List() = %v, want %v", providers, wantProviders)
	}

	if err := Delete("vps-staging"); err != nil {
		t.Fatalf("Delete(vps-staging): %v", err)
	}
	if _, err := Load("vps-staging"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("Load after Delete = %v, want ErrNotFound", err)
	}
	// Deleting a non-existent entry is a no-op.
	if err := Delete("vps-staging"); err != nil {
		t.Fatalf("Delete(missing) = %v, want nil", err)
	}
}

func TestCredstore_RejectsInvalidProviderNames(t *testing.T) {
	t.Setenv("HOME", t.TempDir())

	invalid := []string{
		"",
		".",
		"..",
		"../evil",
		"a/b",
		`a\b`,
		"vps..staging",
		"vps staging",
	}

	for _, name := range invalid {
		if err := Save(name, map[string]string{"host": "1.2.3.4"}); err == nil {
			t.Errorf("Save(%q) succeeded, want error", name)
		}
		if _, err := Load(name); err == nil {
			t.Errorf("Load(%q) succeeded, want error", name)
		}
		if err := Delete(name); err == nil {
			t.Errorf("Delete(%q) succeeded, want error", name)
		}
	}
}
