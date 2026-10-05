package nodeagent

import (
	"context"
	"io"
	"net"
	"net/http"
	"time"

	"github.com/warmbly/warmbly/internal/pkg/publicip"
)

func (a *Agent) publicAddress(ctx context.Context) string {
	if ip, ok := publicip.IPv4(a.cfg.Address); ok {
		return ip
	}
	if time.Since(a.addressChecked) < 10*time.Minute {
		return a.address
	}
	a.addressChecked = time.Now()
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.ipify.org", nil)
	if err != nil {
		return a.address
	}
	// Discovery must use the worker's outbound interface, not an HTTP proxy.
	transport := &http.Transport{}
	if ip := net.ParseIP(a.cfg.Address); ip != nil {
		dialer := &net.Dialer{LocalAddr: &net.TCPAddr{IP: ip}, Timeout: 5 * time.Second}
		transport.DialContext = dialer.DialContext
	}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return a.address
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return a.address
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64))
	if err == nil {
		if ip, ok := publicip.IPv4(string(body)); ok {
			a.address = ip
		}
	}
	return a.address
}
