package nextcore

import (
	"encoding/json"
	"strings"
	"testing"
)

// NextCorePayload crosses a release boundary — CLI writes it, a separately
// versioned daemon reads it — so the version has to survive the trip and a
// mismatch has to be an error rather than a zero value.

func TestValidateSchema_CurrentPayloadIsAccepted(t *testing.T) {
	p := NextCorePayload{SchemaVersion: PayloadSchemaVersion}
	if err := p.ValidateSchema(); err != nil {
		t.Fatalf("current schema version rejected: %v", err)
	}
}

func TestValidateSchema_MissingVersionIsLoud(t *testing.T) {
	var p NextCorePayload // zero value: written before versioning existed
	err := p.ValidateSchema()
	if err == nil {
		t.Fatal("an unversioned payload must be rejected, not silently accepted")
	}
	if !strings.Contains(err.Error(), "no schema_version") {
		t.Errorf("error should name the missing version, got: %v", err)
	}
}

func TestValidateSchema_NewerPayloadNamesTheFix(t *testing.T) {
	p := NextCorePayload{SchemaVersion: PayloadSchemaVersion + 1}
	err := p.ValidateSchema()
	if err == nil {
		t.Fatal("a payload newer than this build must be rejected")
	}
	// The operator needs to be told what to do, not just what happened.
	if !strings.Contains(err.Error(), "upgrade-daemon") {
		t.Errorf("error should name the remedy, got: %v", err)
	}
}

func TestValidateSchema_OlderThanMinimumIsRejected(t *testing.T) {
	if MinSupportedSchemaVersion < 2 {
		t.Skip("no version below the minimum exists yet")
	}
	p := NextCorePayload{SchemaVersion: MinSupportedSchemaVersion - 1}
	if err := p.ValidateSchema(); err == nil {
		t.Fatal("a payload below the minimum supported version must be rejected")
	}
}

// The daemon reads the payload out of JSON in the tarball, so the version has
// to be a real serialized field — not something only present in memory.
func TestSchemaVersionSurvivesJSONRoundTrip(t *testing.T) {
	data, err := json.Marshal(NextCorePayload{SchemaVersion: PayloadSchemaVersion})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"schema_version"`) {
		t.Fatalf("schema_version missing from marshalled payload: %s", data)
	}
	var got NextCorePayload
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.SchemaVersion != PayloadSchemaVersion {
		t.Errorf("SchemaVersion = %d after round trip, want %d", got.SchemaVersion, PayloadSchemaVersion)
	}
	if err := got.ValidateSchema(); err != nil {
		t.Errorf("round-tripped payload rejected: %v", err)
	}
}

// A payload that omits the field entirely (an old CLI's metadata.json) must
// decode to zero and then fail validation — the exact skew scenario.
func TestUnversionedJSONFailsValidation(t *testing.T) {
	var p NextCorePayload
	if err := json.Unmarshal([]byte(`{"app_name":"legacy"}`), &p); err != nil {
		t.Fatal(err)
	}
	if p.SchemaVersion != 0 {
		t.Fatalf("expected zero SchemaVersion, got %d", p.SchemaVersion)
	}
	if err := p.ValidateSchema(); err == nil {
		t.Fatal("legacy metadata.json must be rejected loudly")
	}
}
