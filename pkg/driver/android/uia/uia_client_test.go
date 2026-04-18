package uia

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewUiaClient_UsesRootBasePathWhenAvailable(t *testing.T) {
	var requestBody map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, http.MethodPost, r.Method)
		require.Equal(t, "/session", r.URL.Path)

		err := json.NewDecoder(r.Body).Decode(&requestBody)
		require.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		_, err = w.Write([]byte(`{"value":{"sessionId":"test-session","capabilities":{"platformName":"Android"}}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewUiaClient(server.URL)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "test-session", client.SessionId)
	assert.Equal(t, server.URL, client.RemoteUrl)

	caps, ok := requestBody["capabilities"].(map[string]interface{})
	require.True(t, ok)

	alwaysMatch, ok := caps["alwaysMatch"].(map[string]interface{})
	require.True(t, ok)
	assert.Equal(t, "Android", alwaysMatch["platformName"])
	assert.Equal(t, "UiAutomator2", alwaysMatch["appium:automationName"])

	firstMatch, ok := caps["firstMatch"].([]interface{})
	require.True(t, ok)
	require.Len(t, firstMatch, 1)
}

func TestNewUiaClient_FallsBackToWdHubBasePath(t *testing.T) {
	var requestedPaths []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		w.Header().Set("Content-Type", "application/json")

		switch r.URL.Path {
		case "/session":
			w.WriteHeader(http.StatusNotFound)
			_, err := w.Write([]byte(`{"sessionId":null,"value":{"error":"unknown command","message":"not found"}}`))
			require.NoError(t, err)
		case "/wd/hub/session":
			_, err := w.Write([]byte(`{"value":{"sessionId":"wdhub-session","capabilities":{"platformName":"Android"}}}`))
			require.NoError(t, err)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewUiaClient(server.URL)
	require.NoError(t, err)
	require.NotNil(t, client)
	assert.Equal(t, "wdhub-session", client.SessionId)
	assert.Equal(t, server.URL+"/wd/hub", client.RemoteUrl)
	assert.Equal(t, []string{"/session", "/wd/hub/session"}, requestedPaths)
}

func TestNewUiaClient_ReturnsStructuredError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte(`{"sessionId":null,"value":{"error":"unknown command","message":"The requested resource could not be found"}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client, err := NewUiaClient(server.URL)
	require.Error(t, err)
	assert.Nil(t, client)
	assert.Contains(t, err.Error(), "unknown command")
	assert.Contains(t, err.Error(), "could not be found")
}
