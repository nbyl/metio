package backupmanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFilename(t *testing.T) {
	assert.Equal(t, "manifest-20260817T103000Z.json", Filename("2026-08-17T10:30:00Z"))
	assert.Equal(t, "manifest-20260817T133000Z.json", Filename("2026-08-17T15:30:00+02:00"))
	assert.Equal(t, "manifest-20260817T103000Z.json", Filename("2026-08-17T10:30:00.123456Z"))
	now := time.Now().UTC().Format("20060102T150405Z")
	assert.Equal(t, "manifest-"+now+".json", Filename("not-a-timestamp"))
}

func TestSaveAndLoad(t *testing.T) {
	dir := t.TempDir()
	manifest := Manifest{
		Timestamp:        "2026-08-17T10:30:00Z",
		SnapshotID:       "0123abcd",
		ServerID:         "srv-123",
		RepositoryPrefix: "servers/srv-123/restic/",
		DurationSeconds:  42,
		FileCount:        100,
		RepositorySize:   123456,
		Method:           "restic",
		Status:           StatusCompleted,
	}

	filename, err := Save(dir, manifest)
	require.NoError(t, err)
	assert.Equal(t, "manifest-20260817T103000Z.json", filename)
	assert.FileExists(t, filepath.Join(dir, filename))

	// No temp files may linger after the atomic rename.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	got, err := Load(filepath.Join(dir, filename))
	require.NoError(t, err)
	assert.Equal(t, manifest, *got)

	data, err := os.ReadFile(filepath.Join(dir, filename))
	require.NoError(t, err)
	assert.True(t, strings.HasSuffix(string(data), "\n"))

	var raw map[string]any
	require.NoError(t, json.Unmarshal(data, &raw))
	for _, key := range []string{"timestamp", "snapshot_id", "server_id", "repository_prefix",
		"duration_seconds", "file_count", "repository_size", "method", "status"} {
		assert.Contains(t, raw, key)
	}
}

func TestSaveAccumulatesFiles(t *testing.T) {
	dir := t.TempDir()
	first := Manifest{Timestamp: "2026-08-17T10:30:00Z", SnapshotID: "0123abcd"}
	second := Manifest{Timestamp: "2026-08-18T11:00:00Z", SnapshotID: "deadbeef"}

	filename1, err := Save(dir, first)
	require.NoError(t, err)
	filename2, err := Save(dir, second)
	require.NoError(t, err)

	assert.NotEqual(t, filename1, filename2)
	assert.FileExists(t, filepath.Join(dir, filename1))
	assert.FileExists(t, filepath.Join(dir, filename2))
}

func TestLoadErrors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := Load(filepath.Join(t.TempDir(), "missing.json"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("unparsable content", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "manifest-broken.json")
		require.NoError(t, os.WriteFile(path, []byte("not json"), 0o644))
		_, err := Load(path)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unmarshal")
	})
}

func TestQuarantineRenames(t *testing.T) {
	dir := t.TempDir()

	valid := filepath.Join(dir, "manifest-a.json")
	require.NoError(t, os.WriteFile(valid, []byte("{}"), 0o644))
	require.NoError(t, MarkInvalid(valid))
	assert.NoFileExists(t, valid)
	assert.FileExists(t, valid+".invalid")

	rejected := filepath.Join(dir, "manifest-b.json")
	require.NoError(t, os.WriteFile(rejected, []byte("{}"), 0o644))
	require.NoError(t, MarkRejected(rejected))
	assert.NoFileExists(t, rejected)
	assert.FileExists(t, rejected+".rejected")

	// Quarantined files must no longer match the scan pattern.
	matches, err := filepath.Glob(filepath.Join(dir, FilenamePattern))
	require.NoError(t, err)
	assert.Empty(t, matches)
}
