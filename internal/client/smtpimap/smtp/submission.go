package smtp

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"time"

	"github.com/warmbly/warmbly/internal/client/netbind"
	"github.com/warmbly/warmbly/internal/models"
)

// The two submission ports: 465 speaks TLS from the first byte, 587 upgrades
// with STARTTLS. Every major provider listens on both.
const (
	PortSMTPS      = 465
	PortSubmission = 587
)

// fallbackHeadStart is how long 465 has to itself before 587 is dialled
// alongside it. A reachable 465 connects well inside it, so a healthy network
// never sees the second dial; a blocked one costs this much, not a dial timeout.
const fallbackHeadStart = time.Second

// ErrSMTPSEgressBlocked marks a silent 465 on a worker whose own network is
// known to drop outbound 465, so the mailbox's server is not the one blamed.
var ErrSMTPSEgressBlocked = errors.New("this worker's network blocks outbound port 465")

// dialOutcome is what one dial of the race came back with.
type dialOutcome struct {
	conn net.Conn
	err  error
	port int
}

// Dialed is an open submission socket and the mode to speak on it. Port and
// Security are the mailbox's own unless FellBack is set.
type Dialed struct {
	Conn     net.Conn
	Port     int
	Security string
	FellBack bool
}

// dialTCP opens one TCP connection; a variable so the race can be tested
// without a network that swallows SYNs.
var dialTCP = func(ctx context.Context, local *net.TCPAddr, addr string) (net.Conn, error) {
	return netbind.Dialer(local).DialContext(ctx, "tcp", addr)
}

// DialSubmission opens the socket for a mailbox's SMTP server (host already
// normalized). A mailbox on 465 whose SYN goes unanswered is dialled on 587
// as well, in parallel after a head start, and whichever connects first is
// used: a port that stays silent is a network in the way, not the server,
// and hosts that block outbound 465 mostly leave 587 open. A refused port or
// a name that does not resolve is an answer: one that arrives before 587 is
// dialled is returned at once; one that arrives later leaves 587 to finish,
// and a 587 that connects is used. When nothing connects the error is 465's
// own, marked ErrSMTPSEgressBlocked when this worker has learned that its own
// network drops 465.
func DialSubmission(ctx context.Context, local *net.TCPAddr, host string, port int, security string) (Dialed, error) {
	// Read once: the race's goroutines outlive this call and must not read the package vars.
	dialFn, egress := dialTCP, smtpsEgress
	resolved := models.ResolveSMTPSecurity(security, port)
	if resolved != models.MailSecurityTLS || port != PortSMTPS {
		conn, err := dialFn(ctx, local, models.MailDialAddress(host, port))
		return Dialed{Conn: conn, Port: port, Security: resolved}, err
	}

	raceCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	results := make(chan dialOutcome, 2)
	dial := func(port int) {
		conn, err := dialFn(raceCtx, local, models.MailDialAddress(host, port))
		results <- dialOutcome{conn: conn, err: err, port: port}
	}
	inFlight := 1
	go dial(PortSMTPS)

	// A worker that knows its 465 is blocked gives it no head start.
	blocked := egress.Blocked()
	headStartFor := fallbackHeadStart
	if blocked {
		headStartFor = 0
	}
	headStart := time.NewTimer(headStartFor)
	defer headStart.Stop()
	fallbackStarted := false
	startFallback := func() {
		if fallbackStarted {
			return
		}
		fallbackStarted = true
		inFlight++
		go dial(PortSubmission)
	}

	var primaryErr error
	for inFlight > 0 {
		select {
		case <-headStart.C:
			startFallback()
		case r := <-results:
			inFlight--
			if r.err == nil && r.conn != nil {
				if r.port == PortSMTPS {
					egress.markOpen()
				} else if primaryErr == nil || dialTimedOut(primaryErr) {
					egress.markSilent(host)
				}
				go closeLosers(egress, results, inFlight)
				return Dialed{Conn: r.conn, Port: r.port, Security: securityForPort(r.port), FellBack: r.port != port}, nil
			}
			if r.port != PortSMTPS {
				continue
			}
			primaryErr = r.err
			if primaryErr == nil {
				primaryErr = errors.New("dial returned no connection")
			}
			if dialTimedOut(r.err) {
				startFallback()
			} else if !fallbackStarted {
				return Dialed{}, primaryErr
			}
		}
	}
	if blocked && dialTimedOut(primaryErr) {
		return Dialed{}, fmt.Errorf("%w: %w", ErrSMTPSEgressBlocked, primaryErr)
	}
	return Dialed{}, primaryErr
}

// closeLosers drains the dials still in flight after a winner was taken, so
// a socket that connects late is closed rather than leaked. A late 465 still
// proves the port is open from here.
func closeLosers(egress *egressState, results <-chan dialOutcome, n int) {
	for ; n > 0; n-- {
		if r := <-results; r.conn != nil {
			if r.port == PortSMTPS {
				egress.markOpen()
			}
			r.conn.Close()
		}
	}
}

func securityForPort(port int) string {
	if port == PortSMTPS {
		return models.MailSecurityTLS
	}
	return models.MailSecurityStartTLS
}

// dialTimedOut reports whether a dial ended on a deadline rather than on an
// answer from the network.
func dialTimedOut(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && ne.Timeout() {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, os.ErrDeadlineExceeded)
}
