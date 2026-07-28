package config

import (
	"strings"
	"testing"
)

func TestValidateAppNameAccepts(t *testing.T) {
	for _, name := range []string{
		"ressencesystems",
		"my-app-01",
		"abc",
		strings.Repeat("a", AppNameMaxLen),
	} {
		if err := ValidateAppName(name); err != nil {
			t.Errorf("ValidateAppName(%q) = %v, want nil", name, err)
		}
	}
}

func TestValidateAppNameRejects(t *testing.T) {
	for _, name := range []string{
		"ressencesystems.com",                // a domain, the original incident
		"MyApp",                              // uppercase
		"a_b",                                // underscore
		"my app",                             // space
		"ab",                                 // too short
		"",                                   // missing
		strings.Repeat("a", AppNameMaxLen+1), // too long
		"../../etc",                          // path traversal
	} {
		if err := ValidateAppName(name); err == nil {
			t.Errorf("ValidateAppName(%q) = nil, want error", name)
		}
	}
}

func TestValidateAppNameErrorIsActionable(t *testing.T) {
	err := ValidateAppName("ressencesystems.com")
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()
	// The message is UI: it must carry the offending value, the rule, the
	// slug-vs-domain distinction, and a concrete replacement.
	for _, want := range []string{
		`"ressencesystems.com"`,
		"^[a-z0-9-]+$",
		"app.domain.name",
		"nextdeploy.yml",
		`"ressencesystems-com"`,
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q:\n%s", want, msg)
		}
	}
}

func TestSlugifyAppName(t *testing.T) {
	for in, want := range map[string]string{
		"ressencesystems.com": "ressencesystems-com",
		"My App":              "my-app",
		"a__b":                "a-b",
		".leading.trailing.":  "leading-trailing",
		"x":                   "my-app", // too short to be a valid suggestion
		"":                    "my-app",
	} {
		if got := SlugifyAppName(in); got != want {
			t.Errorf("SlugifyAppName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSlugifyAppNameSuggestionIsItselfValid(t *testing.T) {
	// The suggestion appears in an error telling the user what to type — it had
	// better pass the very rule that produced the error.
	for _, in := range []string{
		"ressencesystems.com",
		"My App",
		strings.Repeat("long.name.", 20),
	} {
		slug := SlugifyAppName(in)
		if err := ValidateAppName(slug); err != nil {
			t.Errorf("SlugifyAppName(%q) = %q which is invalid: %v", in, slug, err)
		}
	}
}
