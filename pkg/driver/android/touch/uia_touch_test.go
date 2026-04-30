package touch

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"trek/internal/engine/decision/shared/types"
	"trek/pkg/driver/android/uia"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUIATouchClick_UsesW3CActions(t *testing.T) {
	var methods []string
	var paths []string
	var payloads []map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		paths = append(paths, r.URL.Path)

		if r.Method == http.MethodPost {
			var payload map[string]interface{}
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
			payloads = append(payloads, payload)
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"value":null}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := uia.NewUiaPageWithSession(server.URL, "session-1")
	touchClient := NewUIATouch(client)

	err := touchClient.Click(types.Point{X: 100, Y: 200})
	require.NoError(t, err)

	assert.Equal(t, []string{http.MethodPost, http.MethodDelete}, methods)
	assert.Equal(t, []string{"/session/session-1/actions", "/session/session-1/actions"}, paths)

	actions, ok := payloads[0]["actions"].([]interface{})
	require.True(t, ok)
	require.Len(t, actions, 1)
}

func TestUIATouchSwipe_UsesActionsEndpoint(t *testing.T) {
	var payload map[string]interface{}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			err := json.NewDecoder(r.Body).Decode(&payload)
			require.NoError(t, err)
		}

		w.Header().Set("Content-Type", "application/json")
		_, err := w.Write([]byte(`{"value":null}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := uia.NewUiaPageWithSession(server.URL, "session-1")
	touchClient := NewUIATouch(client)

	err := touchClient.Swipe(types.Point{X: 10, Y: 20}, types.Point{X: 300, Y: 400}, 10, 1000)
	require.NoError(t, err)

	actions, ok := payload["actions"].([]interface{})
	require.True(t, ok)
	require.Len(t, actions, 1)

	source, ok := actions[0].(map[string]interface{})
	require.True(t, ok)
	sourceActions, ok := source["actions"].([]interface{})
	require.True(t, ok)
	require.Len(t, sourceActions, 5)
}

func TestUIATouchClick_IgnoresUnsupportedReleaseActions(t *testing.T) {
	var methods []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.Header().Set("Content-Type", "application/json")

		if r.Method == http.MethodPost {
			_, err := w.Write([]byte(`{"sessionId":"session-1","value":null}`))
			require.NoError(t, err)
			return
		}

		w.WriteHeader(http.StatusNotFound)
		_, err := w.Write([]byte(`{"sessionId":null,"value":{"error":"unknown command","message":"The requested resource could not be found, or a request was received using an HTTP method that is not supported by the mapped resource"}}`))
		require.NoError(t, err)
	}))
	defer server.Close()

	client := uia.NewUiaPageWithSession(server.URL, "session-1")
	touchClient := NewUIATouch(client)

	err := touchClient.Click(types.Point{X: 100, Y: 200})
	require.NoError(t, err)
	assert.Equal(t, []string{http.MethodPost, http.MethodDelete}, methods)
}
