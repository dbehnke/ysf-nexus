package testhelpers

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestSimulateBridgeActivity(t *testing.T) {
	config := DefaultTestConfig()
	config.RepeaterCount = 0
	config.BridgeCount = 1
	// We will use a local httptest.Server below instead of relying on an external API
	config.APITimeout = 3 * time.Second

	suite := NewIntegrationTestSuite(config)
	if err := suite.Setup(t); err != nil {
		t.Fatalf("Failed to setup test suite: %v", err)
	}
	defer suite.Teardown(t)

	bridge := suite.bridgeNetwork.GetBridge(suite.bridges[0])
	if bridge == nil {
		t.Fatal("No bridge available")
	}

	// Start an httptest server that answers /api/current-talker using the bridge state.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/current-talker" {
			http.NotFound(w, r)
			return
		}

		ct := bridge.GetCurrentTalker()
		if ct == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"callsign": ct.Callsign,
			"location": ct.Location,
		})
	}))
	defer srv.Close()

	// point our API calls at the test server
	apiURL := srv.URL

	// Start a talker
	_, err := bridge.StartTalker("SIM001", "Test Location")
	if err != nil {
		t.Fatalf("Failed to start simulated talker: %v", err)
	}

	// short wait
	time.Sleep(150 * time.Millisecond)

	// Call API
	resp, err := http.Get(apiURL + "/api/current-talker")
	if err != nil {
		t.Logf("API call failed: %v", err)
	} else {
		body, _ := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		var parsed map[string]interface{}
		_ = json.Unmarshal(body, &parsed)
		t.Logf("API Response (during talk): %s", string(body))
	}

	// Send a few packets
	if err := bridge.SendVoicePackets(5); err != nil {
		t.Logf("Failed to send voice packets: %v", err)
	}

	if err := bridge.StopCurrentTalker(); err != nil {
		t.Logf("Failed to stop current talker: %v", err)
	}

	time.Sleep(150 * time.Millisecond)

	resp2, err := http.Get(apiURL + "/api/current-talker")
	if err != nil {
		t.Logf("API call failed (after stop): %v", err)
	} else {
		body2, _ := io.ReadAll(resp2.Body)
		_ = resp2.Body.Close()
		t.Logf("API Response (after stop): %s", string(body2))
	}

	// Start random activity for a short period
	suite.StartRandomActivity(2 * time.Second)
	time.Sleep(3 * time.Second)

	resp3, err := http.Get(apiURL + "/api/current-talker")
	if err != nil {
		t.Logf("API call failed (after random): %v", err)
	} else {
		body3, _ := io.ReadAll(resp3.Body)
		_ = resp3.Body.Close()
		t.Logf("API Response (after random): %s", string(body3))
	}
}
