package safehttp

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"testing"
	"time"

	"golang.org/x/net/dns/dnsmessage"
)

func TestPublicImageAddresses(t *testing.T) {
	t.Setenv("WARMBLY_ALLOW_UNSAFE_WEBHOOK_URLS", "true")
	client := PublicClient(time.Second)
	for _, host := range []string{"localhost", "metadata.google.internal", "127.0.0.1", "10.1.2.3", "172.16.0.1", "192.168.1.1", "169.254.169.254", "100.64.0.1", "0.0.0.0", "[::1]", "[fd00::1]", "[fe80::1]", "[::ffff:127.0.0.1]", "[64:ff9b::7f00:1]", "[2001:db8::1]"} {
		t.Run(host, func(t *testing.T) {
			req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, "https://"+host+"/image.png", nil)
			if err != nil {
				t.Fatal(err)
			}
			res, err := client.Do(req)
			if res != nil {
				_ = res.Body.Close()
			}
			if !errors.Is(err, ErrBlockedAddress) {
				t.Fatalf("private address not blocked at dial time: %v", err)
			}
		})
	}
	for _, address := range []string{"1.1.1.1", "8.8.8.8", "2606:4700:4700::1111"} {
		if IsBlockedIP(net.ParseIP(address)) {
			t.Fatalf("public IP incorrectly blocked: %s", address)
		}
	}
}

func TestPublicImageDNSValidationAndPinning(t *testing.T) {
	server, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = server.Close() }()
	addresses := make(chan [][4]byte)
	go func() {
		buffer := make([]byte, 4096)
		for {
			n, peer, err := server.ReadFrom(buffer)
			if err != nil {
				return
			}
			var message dnsmessage.Message
			if message.Unpack(buffer[:n]) != nil {
				continue
			}
			message.Response = true
			message.RecursionAvailable = true
			for _, question := range message.Questions {
				if question.Type != dnsmessage.TypeA {
					continue
				}
				for _, ip := range <-addresses {
					message.Answers = append(message.Answers, dnsmessage.Resource{Header: dnsmessage.ResourceHeader{Name: question.Name, Type: dnsmessage.TypeA, Class: dnsmessage.ClassINET, TTL: 0}, Body: &dnsmessage.AResource{A: ip}})
				}
			}
			packet, err := message.Pack()
			if err == nil {
				_, _ = server.WriteTo(packet, peer)
			}
		}
	}()
	var dialed []string
	stopped := errors.New("test stopped before connecting to the public network")
	dialer := &net.Dialer{
		Resolver: &net.Resolver{PreferGo: true, Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp", server.LocalAddr().String())
		}},
		ControlContext: func(_ context.Context, _, address string, _ syscall.RawConn) error {
			dialed = append(dialed, address)
			return stopped
		},
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	dial := publicDialContext(dialer, nil)
	go func() { addresses <- [][4]byte{{1, 1, 1, 1}} }()
	if _, err := dial(ctx, "tcp", "image.example.test:443"); !errors.Is(err, stopped) || len(dialed) != 1 || dialed[0] != "1.1.1.1:443" {
		t.Fatalf("must pin the validated IP rather than re-resolve the hostname: %v %v", dialed, err)
	}
	go func() { addresses <- [][4]byte{{127, 0, 0, 1}} }()
	if _, err := dial(ctx, "tcp", "image.example.test:443"); !errors.Is(err, ErrBlockedAddress) || len(dialed) != 1 {
		t.Fatalf("must reject a later private DNS answer: %v", err)
	}
	go func() { addresses <- [][4]byte{{1, 1, 1, 1}, {127, 0, 0, 1}} }()
	if _, err := dial(ctx, "tcp", "image.example.test:443"); !errors.Is(err, ErrBlockedAddress) || len(dialed) != 1 {
		t.Fatalf("must reject mixed public/private DNS answers before dialing: %v", err)
	}
}

func TestPublicImageRedirects(t *testing.T) {
	client := PublicClient(time.Second)
	for _, destination := range []string{"http://cdn.example/image.png", "https://user:secret@cdn.example/image.png"} {
		u, _ := url.Parse(destination)
		if client.CheckRedirect(&http.Request{URL: u}, []*http.Request{{}}) == nil {
			t.Fatal("unsafe redirect accepted")
		}
	}
	u, _ := url.Parse("https://cdn.example/image.png")
	if client.CheckRedirect(&http.Request{URL: u}, []*http.Request{{}}) != nil {
		t.Fatal("public HTTPS redirect refused")
	}
	if client.CheckRedirect(&http.Request{URL: u}, make([]*http.Request, 5)) == nil {
		t.Fatal("redirect cap not enforced")
	}
	if _, err := publicDialContext(&net.Dialer{}, nil)(context.Background(), "tcp", "1.1.1.1:6379"); !errors.Is(err, ErrBlockedAddress) {
		t.Fatal("non-web port accepted")
	}
}
