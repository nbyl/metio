package agentclient

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestSubmitBackupReport(t *testing.T) {
	t.Run("posts report to server endpoint", func(t *testing.T) {
		var (
			gotPath   string
			gotAuth   string
			gotReport BackupReport
		)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			gotPath = r.URL.Path
			gotAuth = r.Header.Get("Authorization")
			require.NoError(t, json.NewDecoder(r.Body).Decode(&gotReport))
			w.WriteHeader(http.StatusCreated)
		}))
		defer server.Close()

		client := New(server.URL, "token-123", "srv-instance")
		report := BackupReport{
			SnapshotID:       "0123abcd",
			RepositoryPrefix: "servers/srv-abcd1234/restic/",
			DurationSeconds:  30,
			FileCount:        120,
			RepositorySize:   654321,
			MinecraftVersion: "1.21.4",
			Status:           "COMPLETED",
		}
		err := client.SubmitBackupReport(context.Background(), "srv-abcd1234", report)
		require.NoError(t, err)

		assert.Equal(t, "/api/servers/srv-abcd1234/backups/report", gotPath)
		assert.Equal(t, "Bearer token-123", gotAuth)
		assert.Equal(t, report, gotReport)
	})

	t.Run("returns HTTPStatusError on bad request", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"invalid repositoryPrefix"}`))
		}))
		defer server.Close()

		client := New(server.URL, "token", "srv-instance")
		err := client.SubmitBackupReport(context.Background(), "srv", BackupReport{})

		require.Error(t, err)
		var statusErr *HTTPStatusError
		require.True(t, errors.As(err, &statusErr))
		assert.Equal(t, http.StatusBadRequest, statusErr.StatusCode)
		assert.Equal(t, "invalid repositoryPrefix", statusErr.Body)
	})
}

func TestHTTPStatusErrorClassification(t *testing.T) {
	permanent := func(err error) bool {
		var statusErr *HTTPStatusError
		return errors.As(err, &statusErr) && statusErr.StatusCode == http.StatusBadRequest
	}
	transient := func(err error) bool {
		var statusErr *HTTPStatusError
		return errors.As(err, &statusErr) && statusErr.StatusCode >= 500
	}

	badRequest := &HTTPStatusError{StatusCode: 400, Body: "nope"}
	serverError := &HTTPStatusError{StatusCode: 503}

	assert.True(t, permanent(badRequest), "400 is a permanent rejection")
	assert.False(t, transient(badRequest))
	assert.True(t, transient(serverError), "5xx is retryable")
	assert.Contains(t, badRequest.Error(), "nope")
	assert.Contains(t, serverError.Error(), "503")

	networkErr := errors.New("request failed: connection refused")
	assert.False(t, permanent(networkErr), "network errors are not HTTPStatusErrors")
}

func TestDecodeErrorFallbacks(t *testing.T) {
	t.Run("unparsable body yields bare status error", func(t *testing.T) {
		err := decodeError(500, stringsReader("not json"))
		var statusErr *HTTPStatusError
		require.True(t, errors.As(err, &statusErr))
		assert.Equal(t, 500, statusErr.StatusCode)
		assert.Empty(t, statusErr.Body)
	})

	t.Run("empty error field yields bare status error", func(t *testing.T) {
		err := decodeError(404, stringsReader(`{"error":""}`))
		var statusErr *HTTPStatusError
		require.True(t, errors.As(err, &statusErr))
		assert.Equal(t, 404, statusErr.StatusCode)
	})
}

func stringsReader(s string) io.Reader {
	return io.NopCloser(strings.NewReader(s))
}
