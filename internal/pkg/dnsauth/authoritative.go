package dnsauth

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"io"
	"net"
	"strings"
	"sync"
	"time"

	"golang.org/x/net/dns/dnsmessage"

	"github.com/warmbly/warmbly/internal/pkg/safehttp"
)

const (
	// nsQueryTimeout bounds one question to one nameserver address.
	nsQueryTimeout = 2 * time.Second
	// maxNSAddrs bounds how many nameserver addresses one question may try.
	maxNSAddrs = 4
	// maxCNAMEHops bounds a CNAME chain (a hosted DMARC or DKIM record is one hop).
	maxCNAMEHops = 5
	// ednsPayload is the UDP size advertised; larger answers come back truncated and go over TCP.
	ednsPayload = 1232
)

// errNoAnswer is any outcome that is not the zone's own authoritative answer.
var errNoAnswer = errors.New("dnsauth: no authoritative answer")

// authoritative asks a name's own nameservers directly, bypassing every cache
// between this host and the zone. It is how a miss from the host's resolver is
// confirmed: that resolver can hold a "no such record" cached from before the
// record was published, drop TXT answers, or fail outright, and none of those
// is evidence the domain is misconfigured. The zone's own servers are the only
// source that cannot be stale.
type authoritative struct {
	ctx context.Context
	// findNS returns the nameserver hosts delegated at exactly name.
	findNS func(ctx context.Context, name string) ([]string, error)
	// addrs returns a nameserver host's addresses.
	addrs func(ctx context.Context, host string) ([]string, error)
	// allow refuses an address a domain owner could aim at this host's network.
	allow func(net.IP) bool
	port  string

	mu    sync.Mutex
	zones map[string]zone
}

// zone is what serversFor learned about one name: whether it is a delegation
// point, and the addresses its nameservers can be reached on.
type zone struct {
	delegated bool
	servers   []string
}

func newAuthoritative(ctx context.Context, resolver *net.Resolver) *authoritative {
	return &authoritative{
		ctx: ctx,
		findNS: func(ctx context.Context, name string) ([]string, error) {
			recs, err := resolver.LookupNS(ctx, rooted(name))
			if err != nil {
				return nil, err
			}
			hosts := make([]string, 0, len(recs))
			for _, r := range recs {
				hosts = append(hosts, r.Host)
			}
			return hosts, nil
		},
		addrs: func(ctx context.Context, host string) ([]string, error) {
			return resolver.LookupHost(ctx, rooted(host))
		},
		allow: func(ip net.IP) bool { return !safehttp.IsBlockedIP(ip) },
		port:  "53",
	}
}

// txt is a lookupFunc answered by the zone's own nameservers. transient is
// true whenever they could not be reached or would not answer, so the caller
// falls back to what the host's resolver said.
func (a *authoritative) txt(name string) ([]string, bool) {
	ctx, cancel := context.WithTimeout(a.ctx, lookupTimeout)
	defer cancel()
	txts, err := a.lookupTXT(ctx, strings.ToLower(strings.TrimSuffix(name, ".")), 0)
	if err != nil {
		return nil, true
	}
	return txts, false
}

func (a *authoritative) lookupTXT(ctx context.Context, name string, hop int) ([]string, error) {
	servers, err := a.serversFor(ctx, name)
	if err != nil {
		return nil, err
	}
	for _, server := range servers {
		txts, cname, err := a.ask(ctx, server, name)
		if err != nil {
			continue
		}
		if cname != "" && len(txts) == 0 {
			// The zone points the name elsewhere (a hosted DMARC or DKIM
			// record); the target lives in another zone with its own servers.
			if hop >= maxCNAMEHops {
				return nil, errNoAnswer
			}
			return a.lookupTXT(ctx, cname, hop+1)
		}
		return txts, nil
	}
	return nil, errNoAnswer
}

// serversFor finds the zone holding name by walking up to the nearest
// delegation, stopping at the registrable domain: a TLD's servers only refer.
func (a *authoritative) serversFor(ctx context.Context, name string) ([]string, error) {
	org := organizationalDomain(name)
	if org == "" {
		return nil, errNoAnswer
	}
	for cut := name; ; {
		z, ok := a.cached(cut)
		if !ok {
			if hosts, err := a.findNS(ctx, cut); err == nil && len(hosts) > 0 {
				z = zone{delegated: true, servers: a.resolveServers(ctx, hosts)}
			}
			a.store(cut, z)
		}
		if z.delegated {
			// The zone's own servers or nobody: a parent only refers.
			if len(z.servers) == 0 {
				return nil, errNoAnswer
			}
			return z.servers, nil
		}
		if cut == org {
			return nil, errNoAnswer
		}
		_, parent, ok := strings.Cut(cut, ".")
		if !ok {
			return nil, errNoAnswer
		}
		cut = parent
	}
}

func (a *authoritative) cached(name string) (zone, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	z, ok := a.zones[name]
	return z, ok
}

func (a *authoritative) store(name string, z zone) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.zones == nil {
		a.zones = map[string]zone{}
	}
	a.zones[name] = z
}

// resolveServers turns nameserver hosts into dialable addresses, IPv4 first
// since a host without IPv6 fails those dials only after trying.
func (a *authoritative) resolveServers(ctx context.Context, hosts []string) []string {
	var v4, v6 []string
	for _, h := range hosts {
		ips, err := a.addrs(ctx, strings.TrimSuffix(h, "."))
		if err != nil {
			continue
		}
		for _, s := range ips {
			ip := net.ParseIP(s)
			if ip == nil || !a.allow(ip) {
				continue
			}
			if ip.To4() != nil {
				v4 = append(v4, net.JoinHostPort(s, a.port))
			} else {
				v6 = append(v6, net.JoinHostPort(s, a.port))
			}
		}
		if len(v4)+len(v6) >= maxNSAddrs {
			break
		}
	}
	out := append(v4, v6...)
	if len(out) > maxNSAddrs {
		out = out[:maxNSAddrs]
	}
	return out
}

// ask puts one TXT question to one server. A definitive answer is either the
// TXT set, a CNAME to follow, or an authoritative "no such record"; anything
// else (a referral, a refusal, a timeout) is errNoAnswer so the next server is tried.
func (a *authoritative) ask(ctx context.Context, server, name string) ([]string, string, error) {
	q, id, err := buildQuery(name)
	if err != nil {
		return nil, "", err
	}
	qctx, cancel := context.WithTimeout(ctx, nsQueryTimeout)
	defer cancel()

	resp, err := exchange(qctx, "udp", server, q)
	if err != nil {
		return nil, "", err
	}
	txts, cname, truncated, err := parseAnswer(resp, id, name)
	if truncated {
		if resp, err = exchange(qctx, "tcp", server, q); err != nil {
			return nil, "", err
		}
		txts, cname, _, err = parseAnswer(resp, id, name)
	}
	return txts, cname, err
}

func buildQuery(name string) ([]byte, uint16, error) {
	var idb [2]byte
	if _, err := rand.Read(idb[:]); err != nil {
		return nil, 0, err
	}
	id := binary.BigEndian.Uint16(idb[:])
	qname, err := dnsmessage.NewName(rooted(name))
	if err != nil {
		return nil, 0, err
	}
	b := dnsmessage.NewBuilder(nil, dnsmessage.Header{ID: id})
	b.EnableCompression()
	if err := b.StartQuestions(); err != nil {
		return nil, 0, err
	}
	if err := b.Question(dnsmessage.Question{Name: qname, Type: dnsmessage.TypeTXT, Class: dnsmessage.ClassINET}); err != nil {
		return nil, 0, err
	}
	if err := b.StartAdditionals(); err != nil {
		return nil, 0, err
	}
	var opt dnsmessage.ResourceHeader
	if err := opt.SetEDNS0(ednsPayload, dnsmessage.RCodeSuccess, false); err != nil {
		return nil, 0, err
	}
	if err := b.OPTResource(opt, dnsmessage.OPTResource{}); err != nil {
		return nil, 0, err
	}
	msg, err := b.Finish()
	return msg, id, err
}

// parseAnswer reads a response to buildQuery. Only an authoritative answer
// counts, so a resolver or a lame server can never stand in for the zone.
func parseAnswer(msg []byte, id uint16, name string) (txts []string, cname string, truncated bool, err error) {
	var p dnsmessage.Parser
	h, err := p.Start(msg)
	if err != nil {
		return nil, "", false, err
	}
	if h.ID != id || !h.Response {
		return nil, "", false, errNoAnswer
	}
	if h.Truncated {
		return nil, "", true, nil
	}
	if !h.Authoritative {
		return nil, "", false, errNoAnswer
	}
	q, err := p.Question()
	if err != nil || !strings.EqualFold(q.Name.String(), rooted(name)) || q.Type != dnsmessage.TypeTXT {
		return nil, "", false, errNoAnswer
	}
	switch h.RCode {
	case dnsmessage.RCodeNameError:
		return nil, "", false, nil
	case dnsmessage.RCodeSuccess:
	default:
		return nil, "", false, errNoAnswer
	}
	if err := p.SkipAllQuestions(); err != nil {
		return nil, "", false, err
	}
	owner := rooted(name)
	for {
		rh, err := p.AnswerHeader()
		if errors.Is(err, dnsmessage.ErrSectionDone) {
			break
		}
		if err != nil {
			return nil, "", false, err
		}
		if !strings.EqualFold(rh.Name.String(), owner) {
			if err := p.SkipAnswer(); err != nil {
				return nil, "", false, err
			}
			continue
		}
		switch rh.Type {
		case dnsmessage.TypeTXT:
			r, err := p.TXTResource()
			if err != nil {
				return nil, "", false, err
			}
			// One record may be split into several strings; they are one value.
			txts = append(txts, strings.Join(r.TXT, ""))
		case dnsmessage.TypeCNAME:
			r, err := p.CNAMEResource()
			if err != nil {
				return nil, "", false, err
			}
			// An in-zone chain is answered in the same message; follow it here.
			cname = strings.ToLower(strings.TrimSuffix(r.CNAME.String(), "."))
			owner = r.CNAME.String()
		default:
			if err := p.SkipAnswer(); err != nil {
				return nil, "", false, err
			}
		}
	}
	return txts, cname, false, nil
}

// exchange sends one query over UDP or TCP (length-prefixed) and returns the reply.
func exchange(ctx context.Context, network, server string, q []byte) ([]byte, error) {
	var d net.Dialer
	conn, err := d.DialContext(ctx, network, server)
	if err != nil {
		return nil, err
	}
	defer func() { _ = conn.Close() }()
	if deadline, ok := ctx.Deadline(); ok {
		_ = conn.SetDeadline(deadline)
	}

	if network == "udp" {
		if _, err := conn.Write(q); err != nil {
			return nil, err
		}
		buf := make([]byte, ednsPayload)
		n, err := conn.Read(buf)
		if err != nil {
			return nil, err
		}
		return buf[:n], nil
	}

	framed := make([]byte, 2+len(q))
	binary.BigEndian.PutUint16(framed, uint16(len(q)))
	copy(framed[2:], q)
	if _, err := conn.Write(framed); err != nil {
		return nil, err
	}
	var size [2]byte
	if _, err := io.ReadFull(conn, size[:]); err != nil {
		return nil, err
	}
	buf := make([]byte, binary.BigEndian.Uint16(size[:]))
	if _, err := io.ReadFull(conn, buf); err != nil {
		return nil, err
	}
	return buf, nil
}

// rooted makes a name fully qualified, so a resolver never retries it under the
// host's search domains: that retry turns a failed lookup into a "not found".
func rooted(name string) string {
	if strings.HasSuffix(name, ".") {
		return name
	}
	return name + "."
}
