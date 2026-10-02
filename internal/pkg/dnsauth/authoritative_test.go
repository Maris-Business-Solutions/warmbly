package dnsauth

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"testing"

	"golang.org/x/net/dns/dnsmessage"
)

// zoneReply is how the fake nameserver answers one name.
type zoneReply struct {
	txt          [][]string // each entry is one record, possibly split into strings
	cname        string
	nxdomain     bool
	notAuth      bool // a resolver or lame server, never the zone
	truncateUDP  bool // too big for UDP: answered in full only over TCP
	servfailOnly bool
}

// fakeZone serves replies over UDP and TCP on one loopback port.
func fakeZone(t *testing.T, replies map[string]zoneReply) string {
	t.Helper()
	var pc net.PacketConn
	var ln net.Listener
	for range 20 {
		var err error
		pc, err = net.ListenPacket("udp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		ln, err = net.Listen("tcp", pc.LocalAddr().String())
		if err == nil {
			break
		}
		_ = pc.Close()
	}
	if ln == nil {
		t.Fatal("could not bind a UDP and TCP port pair")
	}
	t.Cleanup(func() { _ = pc.Close(); _ = ln.Close() })

	answer := func(req []byte, overTCP bool) []byte {
		var p dnsmessage.Parser
		h, err := p.Start(req)
		if err != nil {
			return nil
		}
		q, err := p.Question()
		if err != nil {
			return nil
		}
		r, ok := replies[strings.TrimSuffix(strings.ToLower(q.Name.String()), ".")]
		rh := dnsmessage.Header{ID: h.ID, Response: true, Authoritative: !r.notAuth}
		switch {
		case !ok || r.nxdomain:
			rh.RCode = dnsmessage.RCodeNameError
		case r.servfailOnly:
			rh.RCode = dnsmessage.RCodeServerFailure
		case r.truncateUDP && !overTCP:
			rh.Truncated = true
		}
		b := dnsmessage.NewBuilder(nil, rh)
		_ = b.StartQuestions()
		_ = b.Question(q)
		_ = b.StartAnswers()
		if ok && rh.RCode == dnsmessage.RCodeSuccess && !rh.Truncated {
			hdr := dnsmessage.ResourceHeader{Name: q.Name, Class: dnsmessage.ClassINET, TTL: 60}
			if r.cname != "" {
				_ = b.CNAMEResource(hdr, dnsmessage.CNAMEResource{CNAME: dnsmessage.MustNewName(rooted(r.cname))})
			}
			for _, strs := range r.txt {
				_ = b.TXTResource(hdr, dnsmessage.TXTResource{TXT: strs})
			}
		}
		out, _ := b.Finish()
		return out
	}

	go func() {
		buf := make([]byte, 1500)
		for {
			n, addr, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			_, _ = pc.WriteTo(answer(buf[:n], false), addr)
		}
	}()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func() {
				defer func() { _ = c.Close() }()
				var size [2]byte
				if _, err := io.ReadFull(c, size[:]); err != nil {
					return
				}
				req := make([]byte, binary.BigEndian.Uint16(size[:]))
				if _, err := io.ReadFull(c, req); err != nil {
					return
				}
				out := answer(req, true)
				framed := make([]byte, 2+len(out))
				binary.BigEndian.PutUint16(framed, uint16(len(out)))
				copy(framed[2:], out)
				_, _ = c.Write(framed)
			}()
		}
	}()
	return pc.LocalAddr().String()
}

// testAuthoritative points every zone in delegated at the fake server.
func testAuthoritative(t *testing.T, server string, delegated ...string) *authoritative {
	t.Helper()
	host, port, _ := net.SplitHostPort(server)
	zones := map[string]bool{}
	for _, z := range delegated {
		zones[z] = true
	}
	return &authoritative{
		ctx: context.Background(),
		findNS: func(_ context.Context, name string) ([]string, error) {
			if zones[strings.TrimSuffix(name, ".")] {
				return []string{"ns1." + name}, nil
			}
			return nil, errors.New("no such host")
		},
		addrs: func(context.Context, string) ([]string, error) { return []string{host}, nil },
		allow: func(net.IP) bool { return true },
		port:  port,
	}
}

func TestAuthoritativeAnswers(t *testing.T) {
	long := strings.Repeat("a", 250)
	server := fakeZone(t, map[string]zoneReply{
		"acme.com":                 {txt: [][]string{{"v=spf1 include:_spf.google.com ", "~all"}, {"google-site-verification=x"}}},
		"_dmarc.acme.com":          {cname: "acme.com.dmarc.host.net"},
		"acme.com.dmarc.host.net":  {txt: [][]string{{"v=DMARC1; p=reject"}}},
		"_dmarc.lame.com":          {notAuth: true, txt: [][]string{{"v=DMARC1; p=none"}}},
		"_dmarc.broken.com":        {servfailOnly: true},
		"big.com":                  {truncateUDP: true, txt: [][]string{{long}, {long}, {long}, {long}, {long}, {"v=spf1 -all"}}},
		"k1._domainkey.acme.com":   {txt: [][]string{{"v=DKIM1; k=rsa; p=MIGf"}}},
		"_dmarc.nodata.com":        {},
		"_dmarc.noanswer.acme.com": {nxdomain: true},
	})
	a := testAuthoritative(t, server, "acme.com", "dmarc.host.net", "lame.com", "broken.com", "big.com", "nodata.com")

	tests := []struct {
		name      string
		want      []string
		transient bool
	}{
		{"acme.com", []string{"v=spf1 include:_spf.google.com ~all", "google-site-verification=x"}, false},
		{"_dmarc.acme.com", []string{"v=DMARC1; p=reject"}, false},
		{"k1._domainkey.acme.com", []string{"v=DKIM1; k=rsa; p=MIGf"}, false},
		{"_dmarc.noanswer.acme.com", nil, false},
		{"_dmarc.nodata.com", nil, false},
		{"_dmarc.lame.com", nil, true},
		{"_dmarc.broken.com", nil, true},
		{"_dmarc.undelegated.org", nil, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, transient := a.txt(tt.name)
			if transient != tt.transient {
				t.Fatalf("transient = %v, want %v", transient, tt.transient)
			}
			if strings.Join(got, "|") != strings.Join(tt.want, "|") {
				t.Errorf("txt = %q, want %q", got, tt.want)
			}
		})
	}

	t.Run("truncated answer is retried over TCP", func(t *testing.T) {
		got, transient := a.txt("big.com")
		if transient || !strings.Contains(strings.Join(got, "|"), "v=spf1 -all") {
			t.Errorf("txt = %v (transient %v), want the full set over TCP", len(got), transient)
		}
	})
}

func TestAuthoritativeRefusesPrivateNameservers(t *testing.T) {
	server := fakeZone(t, map[string]zoneReply{"acme.com": {txt: [][]string{{"v=spf1 -all"}}}})
	a := testAuthoritative(t, server, "acme.com")
	a.allow = newAuthoritative(context.Background(), &net.Resolver{}).allow

	for range 2 {
		if _, transient := a.txt("acme.com"); !transient {
			t.Error("a nameserver on loopback was queried; a domain owner must not be able to aim the check at this host's network")
		}
	}
	if z, _ := a.cached("acme.com"); !z.delegated {
		t.Error("a delegation with no usable server was forgotten, so the next lookup would ask the parent")
	}
}

// confirmStub answers like a zone's nameservers and records the questions,
// which DKIM probing asks concurrently.
func confirmStub(records map[string][]string, unreachable bool, asked *[]string) lookupFunc {
	var mu sync.Mutex
	return func(name string) ([]string, bool) {
		mu.Lock()
		*asked = append(*asked, name)
		mu.Unlock()
		if unreachable {
			return nil, true
		}
		return records[name], false
	}
}

func TestCheckConfirmsAResolverMissWithTheZone(t *testing.T) {
	// The host's resolver holds a miss cached from before the records were
	// published; the zone has them.
	zone := map[string][]string{
		"acme.com":                   {"v=spf1 include:_spf.google.com ~all"},
		"_dmarc.acme.com":            {"v=DMARC1; p=none"},
		"google._domainkey.acme.com": {"v=DKIM1; k=rsa; p=MIGf"},
	}
	var asked []string
	l := stubResolver(nil)
	l.confirm = confirmStub(zone, false, &asked)

	res := checkWith("acme.com", nil, l)
	if got := res.State(); got != "passing" {
		t.Fatalf("State() = %q, want passing (%s)", got, res.Summary)
	}
	if !res.DKIMFound {
		t.Error("DKIM was not probed at the zone after the host resolver proved unreliable for this domain")
	}
}

func TestCheckZoneAnswerSettlesATransientResolver(t *testing.T) {
	// The host's resolver fails; the zone answers that the records do not
	// exist, which is a verdict, not an unknown.
	var asked []string
	l := stubResolver(nil, "acme.com", "_dmarc.acme.com")
	l.confirm = confirmStub(nil, false, &asked)

	res := checkWith("acme.com", []string{"s1"}, l)
	if res.LookupError {
		t.Error("LookupError = true, want false: the zone answered")
	}
	if got := res.State(); got != "failing" {
		t.Errorf("State() = %q, want failing", got)
	}
}

func TestCheckUnreachableZoneKeepsTheResolverAnswer(t *testing.T) {
	var asked []string
	l := stubResolver(map[string][]string{"acme.com": {"v=spf1 -all"}}, "_dmarc.acme.com")
	l.confirm = confirmStub(nil, true, &asked)

	res := checkWith("acme.com", []string{"s1"}, l)
	if !res.SPFFound || !res.LookupError {
		t.Errorf("SPFFound=%v LookupError=%v, want the resolver's answer to stand", res.SPFFound, res.LookupError)
	}
	if got := res.State(); got != "unknown" {
		t.Errorf("State() = %q, want unknown", got)
	}
}

func TestCheckFoundRecordsAreNotReconfirmed(t *testing.T) {
	var asked []string
	l := stubResolver(map[string][]string{
		"acme.com":               {"v=spf1 -all"},
		"_dmarc.acme.com":        {"v=DMARC1; p=reject"},
		"s1._domainkey.acme.com": {"v=DKIM1; p=MIGf"},
	})
	l.confirm = confirmStub(nil, false, &asked)

	if res := checkWith("acme.com", []string{"s1"}, l); res.State() != "passing" {
		t.Fatalf("State() = %q, want passing", res.State())
	}
	if len(asked) != 0 {
		t.Errorf("zone asked about %v, want nothing: the resolver already had every record", asked)
	}
}

func TestCheckLooksUpInternationalizedDomainsInASCII(t *testing.T) {
	res := checkWith("mail.bücher.de", []string{"s1"}, stubResolver(map[string][]string{
		"mail.xn--bcher-kva.de":   {"v=spf1 -all"},
		"_dmarc.xn--bcher-kva.de": {"v=DMARC1; p=quarantine"},
	}))
	if got := res.State(); got != "passing" {
		t.Fatalf("State() = %q, want passing (%s)", got, res.Summary)
	}
	if res.Domain != "mail.bücher.de" || res.DMARCDomain != "bücher.de" {
		t.Errorf("Domain=%q DMARCDomain=%q, want the Unicode form the owner typed", res.Domain, res.DMARCDomain)
	}
}

func TestRooted(t *testing.T) {
	if got := rooted("acme.com"); got != "acme.com." {
		t.Errorf("rooted(acme.com) = %q", got)
	}
	if got := rooted("acme.com."); got != "acme.com." {
		t.Errorf("rooted(acme.com.) = %q", got)
	}
}
