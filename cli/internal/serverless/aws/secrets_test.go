package aws

import "testing"

func TestSecretsEqual(t *testing.T) {
	cases := []struct {
		name string
		a, b map[string]string
		want bool
	}{
		{"both nil", nil, nil, true},
		{"empty equal", map[string]string{}, map[string]string{}, true},
		{"same content", map[string]string{"k": "v"}, map[string]string{"k": "v"}, true},
		{"len differs", map[string]string{"k": "v"}, map[string]string{"k": "v", "x": "y"}, false},
		{"value differs", map[string]string{"k": "v"}, map[string]string{"k": "w"}, false},
		{"key missing", map[string]string{"k": "v"}, map[string]string{"x": "v"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := secretsEqual(tc.a, tc.b); got != tc.want {
				t.Errorf("got %v, want %v", got, tc.want)
			}
		})
	}
}
