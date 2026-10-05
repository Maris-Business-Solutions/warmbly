package nodeagent

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/models"
)

func TestPublicAddressPrefersConfiguredIPv4(t *testing.T) {
	a := Agent{cfg: Config{Address: "::ffff:1.1.1.1"}, address: "8.8.8.8", addressChecked: time.Now()}
	if got := a.publicAddress(context.Background()); got != "1.1.1.1" {
		t.Fatalf("expected normalized configured address, got %q", got)
	}
}

func TestPublicAddressCachesDiscoveryAndLeavesMissingAddressUnknown(t *testing.T) {
	for _, address := range []string{"8.8.8.8", ""} {
		a := Agent{cfg: Config{Address: "172.17.0.2"}, address: address, addressChecked: time.Now()}
		if got := a.publicAddress(context.Background()); got != address {
			t.Fatalf("expected cached address %q, got %q", address, got)
		}
	}
}

func TestStoppingHeartbeatSkipsPublicAddressDiscovery(t *testing.T) {
	received := make(chan models.NodeHeartbeat, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var beat models.NodeHeartbeat
		if err := json.NewDecoder(r.Body).Decode(&beat); err != nil {
			t.Errorf("decode farewell: %v", err)
		}
		received <- beat
		if err := json.NewEncoder(w).Encode(models.NodeHeartbeatReply{}); err != nil {
			t.Errorf("encode farewell reply: %v", err)
		}
	}))
	defer server.Close()
	a := New(Config{NodeID: uuid.New(), Role: models.NodeRoleWorker, BaseURL: server.URL, Token: "test-token", Address: "10.0.0.254"})
	a.address = "1.1.1.1"
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if reply := a.beat(ctx, false, true); reply == nil {
		t.Fatal("farewell heartbeat did not reach the backend")
	}
	if !a.addressChecked.IsZero() {
		t.Fatal("farewell must not spend its timeout on IP discovery")
	}
	beat := <-received
	if !beat.Stopping || beat.Address != a.address {
		t.Fatalf("expected stopping heartbeat with cached address: %+v", beat)
	}
}
