// Package backupmanifest defines the JSON manifest format exchanged between
// the mc-backup post-backup hook and the machine-agent through a shared
// directory on the server's data disk (see docs/adr/0004). The hook writes one
// timestamped manifest per successful backup; the agent scans the directory,
// submits every manifest to the controller's backup report API, and deletes it
// only after the controller acknowledges the record.
package backupmanifest

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	// DefaultDir is the in-container location of the shared manifest
	// directory, matching the /manifests volume mounts wired up in
	// server_cloud_config.yml.
	DefaultDir = "/manifests"

	// FilenamePattern matches queued manifests. Quarantined files carry an
	// additional .invalid/.rejected suffix and temporary files end in
	// .tmp, so neither is picked up by a scan using this pattern.
	FilenamePattern = "manifest-*.json"

	// StatusCompleted marks a manifest describing a successful backup.
	StatusCompleted = "COMPLETED"
)

// Manifest is the JSON document stored in a timestamped file
// (e.g. manifest-20260817T103000Z.json). Field names mirror the controller's
// backup report API so the machine-agent can relay manifests without
// re-mapping individual values.
type Manifest struct {
	Timestamp        string `json:"timestamp"`
	SnapshotID       string `json:"snapshot_id"`
	ServerID         string `json:"server_id"`
	RepositoryPrefix string `json:"repository_prefix"`
	DurationSeconds  int64  `json:"duration_seconds"`
	FileCount        int64  `json:"file_count"`
	RepositorySize   int64  `json:"repository_size"`
	Method           string `json:"method"`
	Status           string `json:"status"`
}

// Filename renders a filename-safe timestamp (manifest-<UTC time>.json) from
// an RFC3339 manifest timestamp, falling back to the current time when the
// string cannot be parsed. Each successful backup gets a distinct name so
// multiple snapshot manifests accumulate instead of overwriting each other.
func Filename(ts string) string {
	t := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
		t = parsed.UTC()
	}
	return fmt.Sprintf("manifest-%s.json", t.Format("20060102T150405Z"))
}

// Save atomically writes the manifest to dir/<timestamped file>. It returns
// the filename written.
func Save(dir string, m Manifest) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	filename := Filename(m.Timestamp)
	tmp := filepath.Join(dir, filename+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, filepath.Join(dir, filename)); err != nil {
		return "", err
	}
	return filename, nil
}

// Load reads and unmarshals a single manifest file.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("unmarshal %s: %w", filepath.Base(path), err)
	}
	return &m, nil
}

// MarkInvalid quarantines an unparsable manifest by renaming it so future
// scans no longer pick it up while keeping it around for inspection.
func MarkInvalid(path string) error {
	return os.Rename(path, path+".invalid")
}

// MarkRejected quarantines a manifest the controller permanently rejected
// (HTTP 400 or missing required fields).
func MarkRejected(path string) error {
	return os.Rename(path, path+".rejected")
}
