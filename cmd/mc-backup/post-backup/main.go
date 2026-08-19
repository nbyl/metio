// Command post-backup is the POST_BACKUP_SCRIPT_FILE hook baked into the metio
// mc-backup image. The upstream itzg/mc-backup backup-loop executes it after
// every backup with:
//
//	$1 = backup exit code
//	$2 = path to the backup tool's output log
//
// On success it atomically writes $MANIFEST_DIR/manifest-<timestamp>.json
// describing the most recent backup; each backup produces its own file so a
// slow ingestion process can never miss a snapshot by a file being
// overwritten. The machine-agent mounts the same directory at /manifests. A
// failed backup leaves the previous manifests in place so the dashboard keeps
// reporting the last known-good backup.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

const (
	manifestPrefix     = "mc-backup-manifest: "
	defaultManifestDir = "/manifests"
	resticTimeout      = 60 * time.Second
)

var snapshotSavedRegex = regexp.MustCompile(`snapshot ([a-f0-9]{8,}) saved`)

// Manifest is the JSON document written to a timestamped file in
// $MANIFEST_DIR (e.g. manifest-20260817T103000Z.json).
type Manifest struct {
	Timestamp  string `json:"timestamp"`
	SnapshotID string `json:"snapshot_id"`
	SizeBytes  int64  `json:"size_bytes"`
	Method     string `json:"method"`
}

// resticSnapshot is a single entry in `restic snapshots --json`.
type resticSnapshot struct {
	ID      string `json:"id"`
	Time    string `json:"time"`
	Summary struct {
		TotalSize int64 `json:"total_size"`
	} `json:"summary"`
}

// runRestic executes a restic command and returns its combined output. The
// subprocess inherits the container environment, so RESTIC_REPOSITORY and
// RESTIC_PASSWORD flow through automatically. It is a package var so tests can
// stub it.
var runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, resticTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, "restic", args...)
	return cmd.CombinedOutput()
}

func main() {
	os.Exit(run(os.Args))
}

func run(args []string) int {
	status := 1
	if len(args) > 1 {
		if n, err := strconv.Atoi(args[1]); err == nil {
			status = n
		}
	}
	logPath := ""
	if len(args) > 2 {
		logPath = args[2]
	}

	if status != 0 {
		fmt.Printf("%sbackup failed (exit %d); keeping previous manifest\n", manifestPrefix, status)
		return 0
	}

	snapshotID := parseSnapshotIDFromLog(logPath)

	snap, _ := latestSnapshot(context.Background())
	if snapshotID == "" && snap != nil {
		snapshotID = snap.ID
	}

	if snapshotID == "" {
		fmt.Printf("%scould not resolve snapshot id; skipping manifest update\n", manifestPrefix)
		return 0
	}

	manifest := Manifest{
		Timestamp:  time.Now().UTC().Format(time.RFC3339),
		SnapshotID: snapshotID,
		Method:     backupMethod(),
	}
	if snap != nil {
		if snap.Time != "" {
			manifest.Timestamp = snap.Time
		}
		manifest.SizeBytes = snap.Summary.TotalSize
	}

	manifestDir := os.Getenv("MANIFEST_DIR")
	if manifestDir == "" {
		manifestDir = defaultManifestDir
	}
	filename, err := writeManifest(manifestDir, manifest)
	if err != nil {
		fmt.Printf("%sfailed to write manifest: %v\n", manifestPrefix, err)
		return 1
	}

	fmt.Printf("%swrote %s (snapshot %s)\n", manifestPrefix, filepath.Join(manifestDir, filename), manifest.SnapshotID)
	return 0
}

// parseSnapshotIDFromLog returns the snapshot id of the last line matching
// `snapshot <id> saved` in the backup log, or "" when the log is unreadable or
// contains no such line.
func parseSnapshotIDFromLog(logPath string) string {
	if logPath == "" {
		return ""
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		return ""
	}
	matches := snapshotSavedRegex.FindAllStringSubmatch(string(data), -1)
	if len(matches) == 0 {
		return ""
	}
	return matches[len(matches)-1][1]
}

// latestSnapshot queries the most recent snapshot matching the same tag filter
// the backup applies (BACKUP_NAME + RESTIC_ADDITIONAL_TAGS) so multiple worlds
// sharing one repository stay distinguishable.
func latestSnapshot(ctx context.Context) (*resticSnapshot, error) {
	out, err := runRestic(ctx, "snapshots", "--json", "--latest", "1", "--tag", tagFilter())
	if err != nil {
		return nil, err
	}
	var snaps []resticSnapshot
	if err := json.Unmarshal(out, &snaps); err != nil {
		return nil, err
	}
	if len(snaps) == 0 {
		return nil, fmt.Errorf("no snapshots found")
	}
	return &snaps[0], nil
}

// tagFilter builds the comma-joined restic --tag filter, mirroring how the
// upstream backup-loop constructs restic_tags_filter.
func tagFilter() string {
	tag := os.Getenv("BACKUP_NAME")
	if tag == "" {
		tag = "world"
	}
	if extra := os.Getenv("RESTIC_ADDITIONAL_TAGS"); extra != "" {
		tag = extra + "," + tag
	}
	return tag
}

func backupMethod() string {
	if m := os.Getenv("BACKUP_METHOD"); m != "" {
		return m
	}
	return "restic"
}

// manifestFilename renders a filename-safe timestamp (manifest-<UTC time>.json)
// from an RFC3339 manifest timestamp, falling back to the current time when the
// string cannot be parsed. Each successful backup gets a distinct name so
// multiple snapshot manifests accumulate instead of overwriting each other.
func manifestFilename(ts string) string {
	t := time.Now().UTC()
	if parsed, err := time.Parse(time.RFC3339, ts); err == nil {
		t = parsed.UTC()
	}
	return fmt.Sprintf("manifest-%s.json", t.Format("20060102T150405Z"))
}

// writeManifest atomically writes the manifest to dir/<timestamped file>. It
// returns the filename written.
func writeManifest(dir string, m Manifest) (string, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	data, err := json.Marshal(m)
	if err != nil {
		return "", err
	}
	data = append(data, '\n')

	filename := manifestFilename(m.Timestamp)
	tmp := filepath.Join(dir, filename+".tmp")
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, filepath.Join(dir, filename)); err != nil {
		return "", err
	}
	return filename, nil
}
