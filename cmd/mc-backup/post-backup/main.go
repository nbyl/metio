// Command post-backup is the POST_BACKUP_SCRIPT_FILE hook baked into the metio
// mc-backup image. The upstream itzg/mc-backup backup-loop executes it after
// every backup with:
//
//	$1 = backup exit code
//	$2 = path to the backup tool's output log
//
// On success it atomically writes a timestamped manifest describing the most
// recent backup into $MANIFEST_DIR; each backup produces its own file so a
// slow ingestion process can never miss a snapshot by a file being
// overwritten. The machine-agent mounts the same directory at /manifests and
// relays manifests to the controller's backup report API. A failed backup
// leaves the previous manifests in place so the dashboard keeps reporting the
// last known-good backup.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"time"

	"github.com/nbyl/metio/internal/backupmanifest"
)

const (
	manifestPrefix = "mc-backup-manifest: "
	resticTimeout  = 60 * time.Second
)

var snapshotSavedRegex = regexp.MustCompile(`snapshot ([a-f0-9]{8,}) saved`)

// resticSummary mirrors the summary object restic attaches to snapshots
// (restic >= 0.16 SnapshotSummary schema).
type resticSummary struct {
	BackupStart         string `json:"backup_start"`
	BackupEnd           string `json:"backup_end"`
	TotalFilesProcessed int64  `json:"total_files_processed"`
	TotalBytesProcessed int64  `json:"total_bytes_processed"`
}

// resticSnapshot is a single entry in `restic snapshots --json`.
type resticSnapshot struct {
	ID      string        `json:"id"`
	Time    string        `json:"time"`
	Summary resticSummary `json:"summary"`
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

	serverID := os.Getenv("METIO_SERVER_ID")
	if serverID == "" {
		fmt.Printf("%sMETIO_SERVER_ID not set; cannot build manifest\n", manifestPrefix)
		return 1
	}

	ctx := context.Background()

	snapshotID := parseSnapshotIDFromLog(logPath)

	snap, _ := latestSnapshot(ctx)
	if snapshotID == "" && snap != nil {
		snapshotID = snap.ID
	}

	if snapshotID == "" {
		fmt.Printf("%scould not resolve snapshot id; skipping manifest update\n", manifestPrefix)
		return 0
	}

	manifest := backupmanifest.Manifest{
		Timestamp:        time.Now().UTC().Format(time.RFC3339),
		SnapshotID:       snapshotID,
		ServerID:         serverID,
		RepositoryPrefix: fmt.Sprintf("servers/%s/restic/", serverID),
		Method:           backupMethod(),
		Status:           backupmanifest.StatusCompleted,
	}
	if snap != nil {
		if snap.Time != "" {
			manifest.Timestamp = snap.Time
		}
		manifest.FileCount = snap.Summary.TotalFilesProcessed
		manifest.DurationSeconds = snapshotDurationSeconds(snap.Summary)

		repoSize, err := repositorySize(ctx)
		switch {
		case err == nil:
			manifest.RepositorySize = repoSize
		case snap.Summary.TotalBytesProcessed > 0:
			fmt.Printf("%srepository size lookup failed (%v); falling back to processed bytes\n", manifestPrefix, err)
			manifest.RepositorySize = snap.Summary.TotalBytesProcessed
		default:
			fmt.Printf("%srepository size lookup failed (%v); reporting 0\n", manifestPrefix, err)
		}
	}

	manifestDir := os.Getenv("MANIFEST_DIR")
	if manifestDir == "" {
		manifestDir = backupmanifest.DefaultDir
	}
	filename, err := backupmanifest.Save(manifestDir, manifest)
	if err != nil {
		fmt.Printf("%sfailed to write manifest: %v\n", manifestPrefix, err)
		return 1
	}

	fmt.Printf("%swrote %s (snapshot %s)\n", manifestPrefix, manifestDir+"/"+filename, manifest.SnapshotID)
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

// repositorySize reports the deduplicated size of the repository contents via
// `restic stats --mode raw-data`, which is what the dashboard shows as
// repository size. The result covers the whole repository, not just the
// latest snapshot, matching how restic deduplicates data across backups.
func repositorySize(ctx context.Context) (int64, error) {
	out, err := runRestic(ctx, "stats", "--json", "--mode", "raw-data", "--tag", tagFilter(), "latest")
	if err != nil {
		return 0, err
	}
	var stats struct {
		TotalSize int64 `json:"total_size"`
	}
	if err := json.Unmarshal(out, &stats); err != nil {
		return 0, err
	}
	return stats.TotalSize, nil
}

// snapshotDurationSeconds derives the backup duration from the summary's
// RFC3339 timestamps, falling back to 0 when either timestamp is missing or
// unparsable so a schema drift can never fail the hook.
func snapshotDurationSeconds(summary resticSummary) int64 {
	start, err := time.Parse(time.RFC3339, summary.BackupStart)
	if err != nil {
		return 0
	}
	end, err := time.Parse(time.RFC3339, summary.BackupEnd)
	if err != nil || end.Before(start) {
		return 0
	}
	return int64(end.Sub(start).Seconds())
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
