package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseSnapshotIDFromLog(t *testing.T) {
	t.Run("empty log path", func(t *testing.T) {
		assert.Empty(t, parseSnapshotIDFromLog(""))
	})

	t.Run("unreadable log", func(t *testing.T) {
		assert.Empty(t, parseSnapshotIDFromLog(filepath.Join(t.TempDir(), "missing.log")))
	})

	t.Run("matches last snapshot line", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "backup.log")
		writeLog := func(t *testing.T, content string) string {
			require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
			return path
		}

		log := writeLog(t, "starting backup\nsnapshot 0123abcd saved\nsnapshot 89abcdef1234 saved\ndone\n")
		assert.Equal(t, "89abcdef1234", parseSnapshotIDFromLog(log))
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
			return []byte(`[{"id":"0123abcd","time":"2026-08-17T10:30:00Z","summary":{"total_size":123456}}]`), nil
		}

		snap, err := latestSnapshot(context.Background())
		require.NoError(t, err)
		require.NotNil(t, snap)
		assert.Equal(t, "0123abcd", snap.ID)
		assert.Equal(t, "2026-08-17T10:30:00Z", snap.Time)
		assert.Equal(t, int64(123456), snap.Summary.TotalSize)
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

func TestManifestFilename(t *testing.T) {
	assert.Equal(t, "manifest-20260817T103000Z.json", manifestFilename("2026-08-17T10:30:00Z"))
	assert.Equal(t, "manifest-20260817T133000Z.json", manifestFilename("2026-08-17T15:30:00+02:00"))
	assert.Equal(t, "manifest-20260817T103000Z.json", manifestFilename("2026-08-17T10:30:00.123456Z"))
	now := time.Now().UTC().Format("20060102T150405Z")
	assert.Equal(t, "manifest-"+now+".json", manifestFilename("not-a-timestamp"))
}

func TestWriteManifest(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{
		Timestamp:  "2026-08-17T10:30:00Z",
		SnapshotID: "0123abcd",
		SizeBytes:  123456,
		Method:     "restic",
	}

	filename, err := writeManifest(dir, manifest)
	require.NoError(t, err)
	assert.Equal(t, "manifest-20260817T103000Z.json", filename)

	path := filepath.Join(dir, filename)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(data), "\n"))

	var got Manifest
	require.NoError(t, json.Unmarshal(data, &got))
	assert.Equal(t, manifest, got)

	// A second successful backup must not overwrite the first.
	manifest.Timestamp = "2026-08-18T11:00:00Z"
	manifest.SnapshotID = "deadbeef"
	filename2, err := writeManifest(dir, manifest)
	require.NoError(t, err)
	assert.NotEqual(t, filename, filename2)
	require.FileExists(t, path)
	require.FileExists(t, filepath.Join(dir, filename2))
}

func TestRun(t *testing.T) {
	orig := runRestic
	t.Cleanup(func() { runRestic = orig })

	t.Run("non-zero status keeps previous manifest", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			t.Fatal("restic must not be called on failed backup")
			return nil, nil
		}
		assert.Equal(t, 0, run([]string{"post-backup", "1", "/dev/null"}))
		assertEmptyDir(t, dir)
	})

	t.Run("writes manifest from log snapshot id", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		logPath := filepath.Join(t.TempDir(), "backup.log")
		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 0123abcd saved\n"), 0o644))

		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return []byte(`[{"id":"cafebabe","time":"2026-08-17T10:30:00Z","summary":{"total_size":42}}]`), nil
		}

		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		var got Manifest
		requireFileJSON(t, filepath.Join(dir, "manifest-20260817T103000Z.json"), &got)
		assert.Equal(t, "2026-08-17T10:30:00Z", got.Timestamp)
		assert.Equal(t, "0123abcd", got.SnapshotID)
		assert.Equal(t, int64(42), got.SizeBytes)
		assert.Equal(t, "restic", got.Method)
	})

	t.Run("falls back to restic snapshot id", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return []byte(`[{"id":"cafebabe","time":"2026-08-17T10:30:00Z","summary":{"total_size":7}}]`), nil
		}

		assert.Equal(t, 0, run([]string{"post-backup", "0", "/dev/null"}))

		var got Manifest
		requireFileJSON(t, filepath.Join(dir, "manifest-20260817T103000Z.json"), &got)
		assert.Equal(t, "cafebabe", got.SnapshotID)
	})

	t.Run("restic failure falls back to log id", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		logPath := filepath.Join(t.TempDir(), "backup.log")
		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 0123abcd saved\n"), 0o644))

		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, os.ErrNotExist
		}

		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		files := listManifestFiles(t, dir)
		require.Len(t, files, 1)
		var got Manifest
		requireFileJSON(t, files[0], &got)
		assert.Equal(t, "0123abcd", got.SnapshotID)
		assert.Equal(t, int64(0), got.SizeBytes)
	})

	t.Run("skips manifest when no snapshot id resolves", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return nil, os.ErrNotExist
		}

		assert.Equal(t, 0, run([]string{"post-backup", "0", "/dev/null"}))
		assertEmptyDir(t, dir)
	})

	t.Run("accumulates one file per successful backup", func(t *testing.T) {
		dir := t.TempDir()
		t.Setenv("MANIFEST_DIR", dir)
		logPath := filepath.Join(t.TempDir(), "backup.log")
		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 11111111 saved\n"), 0o644))

		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return []byte(`[{"id":"aaaa0001","time":"2026-08-17T10:30:00Z","summary":{"total_size":1}}]`), nil
		}
		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		require.NoError(t, os.WriteFile(logPath, []byte("snapshot 22222222 saved\n"), 0o644))
		runRestic = func(ctx context.Context, args ...string) ([]byte, error) {
			return []byte(`[{"id":"aaaa0002","time":"2026-08-18T11:00:00Z","summary":{"total_size":2}}]`), nil
		}
		assert.Equal(t, 0, run([]string{"post-backup", "0", logPath}))

		files, err := os.ReadDir(dir)
		require.NoError(t, err)
		require.Len(t, files, 2)
		paths := []string{files[0].Name(), files[1].Name()}
		assert.Contains(t, paths, "manifest-20260817T103000Z.json")
		assert.Contains(t, paths, "manifest-20260818T110000Z.json")
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
	paths, err := filepath.Glob(filepath.Join(dir, "manifest-*.json"))
	require.NoError(t, err)
	return paths
}

func requireFileJSON(t *testing.T, path string, v any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, v))
}
