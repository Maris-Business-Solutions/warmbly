package nodeagent

import (
	"context"
	"testing"
	"time"
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
