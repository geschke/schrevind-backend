package restore

import (
	"strings"
	"testing"
)

func TestLoadReportsJSONSyntaxLine(t *testing.T) {
	data := []byte(`{
  "format": "schrevind-export",
  "version": 1,
  "data": {
`)

	_, err := Load(data, nil)
	if err == nil {
		t.Fatalf("Load() error = nil, want INVALID_BACKUP_FORMAT")
	}
	msg := err.Error()
	for _, want := range []string{"INVALID_BACKUP_FORMAT:", "line 4", `"data": {`} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Load() error = %q, want to contain %q", msg, want)
		}
	}
}

func TestLoadReportsJSONTypeLine(t *testing.T) {
	data := []byte(`{
  "format": "schrevind-export",
  "version": 1,
  "exported_at": "2026-05-03T08:05:46Z",
  "data": {
    "users": [
      {
        "ID": 1,
        "Settings": "{}"
      }
    ]
  }
}`)

	_, err := Load(data, nil)
	if err == nil {
		t.Fatalf("Load() error = nil, want INVALID_BACKUP_FORMAT")
	}
	msg := err.Error()
	for _, want := range []string{"INVALID_BACKUP_FORMAT:", "Settings", "line 9", `"Settings": "{}"`} {
		if !strings.Contains(msg, want) {
			t.Fatalf("Load() error = %q, want to contain %q", msg, want)
		}
	}
}

func TestLoadReportsUnsupportedVersion(t *testing.T) {
	data := []byte(`{
  "format": "schrevind-export",
  "version": 2,
  "data": {}
}`)

	_, err := Load(data, nil)
	if err == nil {
		t.Fatalf("Load() error = nil, want INVALID_BACKUP_FORMAT")
	}
	msg := err.Error()
	if !strings.Contains(msg, "INVALID_BACKUP_FORMAT: unsupported version 2, want 1") {
		t.Fatalf("Load() error = %q", msg)
	}
}
