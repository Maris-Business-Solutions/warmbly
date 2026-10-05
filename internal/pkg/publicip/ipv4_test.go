package publicip

import "testing"

func TestIPv4(t *testing.T) {
	for _, input := range []string{"10.0.0.1", "127.0.0.1", "0.1.2.3", "100.64.0.1", "198.51.100.3", "2001:4860:4860::8888", "bad", "169.254.2.3"} {
		if ip, ok := IPv4(input); ok {
			t.Fatalf("non-public IPv4 %q accepted as %q", input, ip)
		}
	}
	if ip, ok := IPv4(" ::ffff:1.1.1.1\n"); !ok || ip != "1.1.1.1" {
		t.Fatalf("mapped public IPv4 not normalized: %q, %v", ip, ok)
	}
}
