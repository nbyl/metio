package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/nbyl/metio/internal/backupmanifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// stubRestic installs a runRestic stub that dispatches on the restic
// subcommand and fails the test on unexpected invocations.
func stubRestic(t *testing.T, snapshotsJSON string, statsJSON string) {
	t.Helper()
	orig := runRestic
	t.Cleanup(func() { runRestic = orig })
	runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
		require.NotEmpty(t, args)
		switch args[0] {
		case "snapshots":
			return []byte(snapshotsJSON), nil
		case "stats":
			return []byte(statsJSON), nil
		default:
			t.Fatalf("unexpected restic command: %v", args)
			return nil, nil
		}
	}
}

func TestParseSnapshotIDFromLog(t *testing.T) {
	t.Run("empty log path", func(t *testing.T) {
		assert.Empty(t, parseSnapshotIDFromLog(""))
	})

	t.Run("unreadable log", func(t *testing.T) {
		assert.Empty(t, parseSnapshotIDFromLog(filepath.Join(t.TempDir(), "missing.log")))
	})

	t.Run("matches last snapshot line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "backup.log")
		log := "starting backup\nsnapshot 0123abcd saved\nsnapshot 89abcdef1234 saved\ndone\n"
		require.NoError(t, os.WriteFile(path, []byte(log), 0o644))
		assert.Equal(t, "89abcdef1234", parseSnapshotIDFromLog(path))
	})

	t.Run("no snapshot line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "backup.log")
		require.NoError(t, os.WriteFile(path, []byte("nothing here\n"), 0o644))
		assert.Empty(t, parseSnapshotIDFromLog(path))
	})
}

func TestLatestSnapshot(t *testing.T) {
	orig := runRestic
	t.Cleanup(func() { runRestic = orig })

	t.Run("parses snapshot json", func(t *testing.T) {
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			assert.Equal(t, []string{"snapshots", "--json", "--latest", "1", "--tag", "world"}, args)
			return []byte(`[{"id":"0123abcd","time":"2026-08-17T10:30:00Z","summary":{"backup_start":"2026-08-17T10:30:00Z","backup_end":"2026-08-17T10:31:00Z","total_files_processed":120,"total_bytes_processed":4096}}]`), nil
		}

		snap, err := latestSnapshot(context.Background())
		require.NoError(t, err)
		require.NotNil(t, snap)
		assert.Equal(t, "0123abcd", snap.ID)
		assert.Equal(t, "2026-08-17T10:30:00Z", snap.Time)
		assert.Equal(t, int64(120), snap.Summary.TotalFilesProcessed)
		assert.Equal(t, int64(60), snapshotDurationSeconds(snap.Summary))
	})

	t.Run("restic error", func(t *testing.T) {
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, os.ErrNotExist
		}
		_, err := latestSnapshot(context.Background())
		require.Error(t, err)
	})

	t.Run("no snapshots", func(t *testing.T) {
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return []byte(`[]`), nil
		}
		_, err := latestSnapshot(context.Background())
		require.Error(t, err)
	})
}

func TestRepositorySize(t *testing.T) {
	orig := runRestic
	t.Cleanup(func() { runRestic = orig })

	t.Run("parses stats output", func(t *testing.T) {
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			assert.Equal(t, []string{"stats", "--json", "--mode", "raw-data", "--tag", "world", "latest"}, args)
			return []byte(`{"total_size":654321,"total_blob_count":42}`), nil
		}

		size, err := repositorySize(context.Background())
		require.NoError(t, err)
		assert.Equal(t, int64(654321), size)
	})

	t.Run("propagates restic errors", func(t *testing.T) {
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, os.ErrPermission
		}
		_, err := repositorySize(context.Background())
		require.ErrorIs(t, err, os.ErrPermission)
	})
}

func TestSnapshotDurationSeconds(t *testing.T) {
	assert.Equal(t, int64(90),
		snapshotDurationSeconds(resticSummary{BackupStart: "2026-08-17T10:30:00Z", BackupEnd: "2026-08-17T10:31:30Z"}))
	assert.Equal(t, int64(0), snapshotDurationSeconds(resticSummary{}))
	assert.Equal(t, int64(0),
		snapshotDurationSeconds(resticSummary{BackupStart: "garbage", BackupEnd: "2026-08-17T10:31:00Z"}))

	// end before start must not produce a negative duration.
	assert.Equal(t, int64(0),
		snapshotDurationSeconds(resticSummary{BackupStart: "2026-08-17T10:31:00Z", BackupEnd: "2026-08-17T10:30:00Z"}))
}

func TestTagFilter(t *testing.T) {
	t.Run("defaults to world", func(t *testing.T) {
		t.Setenv("BACKUP_NAME", "")
		t.Setenv("RESTIC_ADDITIONAL_TAGS", "")
		assert.Equal(t, "world", tagFilter())
	})

	t.Run("with backup name", func(t *testing.T) {
		t.Setenv("BACKUP_NAME", "smith-world")
		t.Setenv("RESTIC_ADDITIONAL_TAGS", "")
		assert.Equal(t, "smith-world", tagFilter())
	})

	t.Run("prepends additional tags", func(t *testing.T) {
		t.Setenv("BACKUP_NAME", "smith-world")
		t.Setenv("RESTIC_ADDITIONAL_TAGS", "minecraft,server1")
		assert.Equal(t, "minecraft,server1,smith-world", tagFilter())
	})
}

func TestBackupMethod(t *testing.T) {
	t.Run("defaults to restic", func(t *testing.T) {
		t.Setenv("BACKUP_METHOD", "")
		assert.Equal(t, "restic", backupMethod())
	})
	t.Run("honors env", func(t *testing.T) {
		t.Setenv("BACKUP_METHOD", "restic-secret")
		assert.Equal(t, "restic-secret", backupMethod())
	})
}

func TestRun(t *testing.T) {
	t.Run("non-zero status keeps previous manifest", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		t.Setenv("METIO_SERVER_ID", "srv-1")
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			t.Fatal("restic must not be called on failed backup")
			return nil, nil
		}
		assert.Equal(t, 0, run([]string{"post-backup", "1", "/dev/null"}))
		assertEmptyDir(t, dir)
	})

	t.Run("fails without METIO_SERVER_ID", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		t.Setenv("METIO_SERVER_ID", "")
		stubRestic(t, `[]`, `{}`)
		assert.Equal(t, 1, run([]string{"post-backup", "0", "/dev/null"}))
		assertEmptyDir(t, dir)
	})

	t.Run("writes manifest from log snapshot id", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		t.Setenv("METIO_SERVER_ID", "srv-abcd1234")
		logPath := filepath.Join(t.TempDir(), "backup.log")
		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 0123abcd saved\n"), 0o644))

		stubRestic(t,
			`[{"id":"cafebabe","time":"2026-08-17T10:30:00Z","summary":{"backup_start":"2026-08-17T10:29:30Z","backup_end":"2026-08-17T10:30:00Z","total_files_processed":7,"total_bytes_processed":2048}}]`,
			`{"total_size":9999}`)

		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		var got backupmanifest.Manifest
		requireFileJSON(t, filepath.Join(dir, "manifest-20260817T103000Z.json"), &got)
		assert.Equal(t, "2026-08-17T10:30:00Z", got.Timestamp)
		assert.Equal(t, "0123abcd", got.SnapshotID)
		assert.Equal(t, "srv-abcd1234", got.ServerID)
		assert.Equal(t, "servers/srv-abcd1234/restic/", got.RepositoryPrefix)
		assert.Equal(t, int64(30), got.DurationSeconds)
		assert.Equal(t, int64(7), got.FileCount)
		assert.Equal(t, int64(9999), got.RepositorySize)
		assert.Equal(t, "restic", got.Method)
		assert.Equal(t, backupmanifest.StatusCompleted, got.Status)
	})

	t.Run("falls back to restic snapshot id", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		t.Setenv("METIO_SERVER_ID", "srv-1")
		stubRestic(t,
			`[{"id":"cafebabe","time":"2026-08-17T10:30:00Z","summary":{"total_files_processed":3}}]`,
			`{}`)

		assert.Equal(t, 0, run([]string{"post-backup", "0", "/dev/null"}))

		var got backupmanifest.Manifest
		requireFileJSON(t, filepath.Join(dir, "manifest-20260817T103000Z.json"), &got)
		assert.Equal(t, "cafebabe", got.SnapshotID)
		assert.Equal(t, "servers/srv-1/restic/", got.RepositoryPrefix)
		assert.Equal(t, int64(0), got.DurationSeconds)
	})

	t.Run("repository size falls back to processed bytes", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		t.Setenv("METIO_SERVER_ID", "srv-1")
		logPath := filepath.Join(t.TempDir(), "backup.log")
		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 0123abcd saved\n"), 0o644))

		orig := runRestic
		t.Cleanup(func() { runRestic = orig })
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			if args[0] == "snapshots" {
				return []byte(`[{"id":"0123abcd","time":"2026-08-17T10:30:00Z","summary":{"total_files_processed":5,"total_bytes_processed":777}}]`), nil
			}
			return nil, os.ErrNotExist // stats lookup fails
		}

		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		var got backupmanifest.Manifest
		requireFileJSON(t, filepath.Join(dir, "manifest-20260817T103000Z.json"), &got)
		assert.Equal(t, int64(777), got.RepositorySize)
	})

	t.Run("restic failure falls back to log id", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		t.Setenv("METIO_SERVER_ID", "srv-1")
		logPath := filepath.Join(t.TempDir(), "backup.log")
		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 0123abcd saved\n"), 0o644))

		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, os.ErrNotExist
		}

		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		files := listManifestFiles(t, dir)
		require.Len(t, files, 1)
		var got backupmanifest.Manifest
		requireFileJSON(t, files[0], &got)
		assert.Equal(t, "0123abcd", got.SnapshotID)
		assert.Equal(t, int64(0), got.RepositorySize)
	})

	t.Run("skips manifest when no snapshot id resolves", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		t.Setenv("METIO_SERVER_ID", "srv-1")
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return []byte(`[]`), nil
		}

		assert.Equal(t, 0, run([]string{"post-backup", "0", "/dev/null"}))
		assertEmptyDir(t, dir)
	})

	t.Run("accumulates one file per successful backup", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		t.Setenv("METIO_SERVER_ID", "srv-1")
		logPath := filepath.Join(t.TempDir(), "backup.log")

		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 11111111 saved\n"), 0o644))
		stubRestic(t,
			`[{"id":"aaaa0001","time":"2026-08-17T10:30:00Z","summary":{"total_files_processed":1}}]`,
			`{}`)
		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 22222222 saved\n"), 0o644))
		stubRestic(t,
			`[{"id":"aaaa0002","time":"2026-08-18T11:00:00Z","summary":{"total_files_processed":2}}]`,
			`{}`)
		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, entries, 2)
		names := []string{entries[0].Name(), entries[1].Name()}
		assert.Contains(t, names, "manifest-20260817T103000Z.json")
		assert.Contains(t, names, "manifest-20260818T110000Z.json")
	})
}

func assertEmptyDir(t *testing.T, dir string) {
	t.Helper()
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func listManifestFiles(t *testing.T, dir string) []string {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, backupmanifest.FilenamePattern))
	require.NoError(t, err)
	return paths
}

func requireFileJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, v))
}
