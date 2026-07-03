package main

import (
	"encoding/json"
	"sort"
	"testing"
)

func TestRevalidateMessage_Unmarshal(t *testing.T) {
	tests := []struct {
		name     string
		body     string
		wantTag  string
		wantPath string
		wantErr  bool
	}{
		{name: "tag and path", body: `{"tag":"post-1","path":"/blog/x"}`, wantTag: "post-1", wantPath: "/blog/x"},
		{name: "path only", body: `{"path":"/about"}`, wantPath: "/about"},
		{name: "tag only", body: `{"tag":"home"}`, wantTag: "home"},
		{name: "empty object", body: `{}`},
		{name: "malformed", body: `{not json`, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var msg RevalidateMessage
			err := json.Unmarshal([]byte(tt.body), &msg)
			if (err != nil) != tt.wantErr {
				t.Fatalf("Unmarshal(%q) err = %v, wantErr %v", tt.body, err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if msg.Tag != tt.wantTag {
				t.Errorf("Tag = %q, want %q", msg.Tag, tt.wantTag)
			}
			if msg.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", msg.Path, tt.wantPath)
			}
		})
	}
}

func TestTagPathMap_Unmarshal(t *testing.T) {
	body := `{"tags":{"t":["/a","/b"]},"intervals":{"t":60}}`
	var m TagPathMap
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if got := m.Tags["t"]; len(got) != 2 || got[0] != "/a" || got[1] != "/b" {
		t.Errorf(`Tags["t"] = %v, want ["/a" "/b"]`, got)
	}
	if m.Intervals["t"] != 60 {
		t.Errorf(`Intervals["t"] = %d, want 60`, m.Intervals["t"])
	}
}

func TestCollectPaths(t *testing.T) {
	tests := []struct {
		name   string
		bodies []string
		tagMap TagPathMap
		want   []string // compared as a set (sorted)
	}{
		{
			name:   "single path",
			bodies: []string{`{"path":"/a"}`},
			want:   []string{"/a"},
		},
		{
			name:   "tag expands",
			bodies: []string{`{"tag":"t"}`},
			tagMap: TagPathMap{Tags: map[string][]string{"t": {"/x", "/y"}}},
			want:   []string{"/x", "/y"},
		},
		{
			name:   "path and tag union",
			bodies: []string{`{"path":"/a","tag":"t"}`},
			tagMap: TagPathMap{Tags: map[string][]string{"t": {"/x"}}},
			want:   []string{"/a", "/x"},
		},
		{
			name:   "dedupe across records",
			bodies: []string{`{"path":"/a"}`, `{"path":"/a"}`},
			want:   []string{"/a"},
		},
		{
			name:   "dedupe path vs tag overlap",
			bodies: []string{`{"path":"/x","tag":"t"}`},
			tagMap: TagPathMap{Tags: map[string][]string{"t": {"/x"}}},
			want:   []string{"/x"},
		},
		{
			name:   "unknown tag",
			bodies: []string{`{"tag":"missing"}`},
			tagMap: TagPathMap{Tags: map[string][]string{"t": {"/x"}}},
			want:   []string{},
		},
		{
			name:   "nil tag map ignores tag without panic",
			bodies: []string{`{"tag":"t","path":"/a"}`},
			tagMap: TagPathMap{}, // Tags nil
			want:   []string{"/a"},
		},
		{
			name:   "malformed record skipped",
			bodies: []string{`{bad`, `{"path":"/a"}`},
			want:   []string{"/a"},
		},
		{
			name:   "empty batch",
			bodies: []string{},
			want:   []string{},
		},
		{
			name:   "empty path string ignored",
			bodies: []string{`{"path":""}`},
			want:   []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := collectPaths(tt.bodies, tt.tagMap)
			// Map iteration order is random — compare as sets.
			sort.Strings(got)
			want := append([]string(nil), tt.want...)
			sort.Strings(want)
			if len(got) != len(want) {
				t.Fatalf("collectPaths = %v, want %v", got, want)
			}
			for i := range want {
				if got[i] != want[i] {
					t.Fatalf("collectPaths = %v, want %v", got, want)
				}
			}
		})
	}
}
