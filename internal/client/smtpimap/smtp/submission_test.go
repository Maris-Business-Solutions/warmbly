package smtp

import (
	"context"
	"errors"
	"net"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/warmbly/warmbly/internal/models"
)

type fakeTimeout struct{}

func (fakeTimeout) Error() string   { return "i/o timeout" }
func (fakeTimeout) Timeout() bool   { return true }
func (fakeTimeout) Temporary() bool { return false }

// swapDial installs a dialer for the test and restores the real one after.
func swapDial(t *testing.T, fn func(ctx context.Context, addr string) (net.Conn, error)) {
	t.Helper()
	prev, prevEgress := dialTCP, smtpsEgress
	dialTCP = func(ctx context.Context, _ *net.TCPAddr, addr string) (net.Conn, error) { return fn(ctx, addr) }
	smtpsEgress = &egressState{}
	t.Cleanup(func() { dialTCP, smtpsEgress = prev, prevEgress })
}

// A 465 whose SYN is swallowed is taken on 587 after the head start, well
// before the dial timeout, and the caller is told which socket it holds.
func TestDialSubmission_SilentSMTPSFallsBackTo587(t *testing.T) {
	var dials int32
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		atomic.AddInt32(&dials, 1)
		if strings.HasSuffix(addr, ":465") {
			<-ctx.Done()
			return nil, fakeTimeout{}
		}
		a, b := net.Pipe()
		go func() { _ = b }()
		return a, nil
	})
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	d, err := DialSubmission(ctx, nil, "mail.example.com", 465, models.MailSecurityTLS)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer d.Conn.Close()
	if !d.FellBack || d.Port != 587 || d.Security != models.MailSecurityStartTLS {
		t.Fatalf("got %+v, want the 587 STARTTLS socket", d)
	}
	if took := time.Since(start); took > 3*time.Second {
		t.Fatalf("fallback took %s, should not wait for the dial timeout", took)
	}
	if n := atomic.LoadInt32(&dials); n != 2 {
		t.Fatalf("%d dials, want 465 and 587", n)
	}
}

// A 465 that answers is used as is, and 587 is never dialled.
func TestDialSubmission_ReachableSMTPSIsKept(t *testing.T) {
	var dials int32
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		atomic.AddInt32(&dials, 1)
		a, _ := net.Pipe()
		return a, nil
	})
	d, err := DialSubmission(context.Background(), nil, "mail.example.com", 465, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer d.Conn.Close()
	if d.FellBack || d.Port != 465 || d.Security != models.MailSecurityTLS {
		t.Fatalf("got %+v, want the mailbox's own 465 socket", d)
	}
	if n := atomic.LoadInt32(&dials); n != 1 {
		t.Fatalf("%d dials, want only 465", n)
	}
}

// A refusal is an answer: it is returned at once and 587 is not tried.
func TestDialSubmission_RefusedSMTPSIsNotRetried(t *testing.T) {
	var dials int32
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		atomic.AddInt32(&dials, 1)
		return nil, &net.OpError{Op: "dial", Err: syscall.ECONNREFUSED}
	})
	_, err := DialSubmission(context.Background(), nil, "mail.example.com", 465, models.MailSecurityTLS)
	if !errors.Is(err, syscall.ECONNREFUSED) {
		t.Fatalf("got %v, want the refusal", err)
	}
	if n := atomic.LoadInt32(&dials); n != 1 {
		t.Fatalf("%d dials, want only 465", n)
	}
}

// When neither port connects the error is 465's own, so the message names
// the port the mailbox was configured with.
func TestDialSubmission_BothSilentReportsSMTPS(t *testing.T) {
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		<-ctx.Done()
		return nil, fakeTimeout{}
	})
	ctx, cancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)
	defer cancel()
	d, err := DialSubmission(ctx, nil, "mail.example.com", 465, models.MailSecurityTLS)
	if err == nil || d.Conn != nil {
		t.Fatalf("got %+v, %v; want a timeout", d, err)
	}
	if !dialTimedOut(err) {
		t.Fatalf("got %v, want a timeout", err)
	}
}

// Any other port or mode is a single plain dial.
func TestDialSubmission_OtherPortsDialOnce(t *testing.T) {
	var dials int32
	swapDial(t, func(ctx context.Context, addr string) (net.Conn, error) {
		atomic.AddInt32(&dials, 1)
		a, _ := net.Pipe()
		return a, nil
	})
	d, err := DialSubmission(context.Background(), nil, "mail.example.com", 587, "")
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer d.Conn.Close()
	if d.FellBack || d.Port != 587 || d.Security != models.MailSecurityStartTLS || atomic.LoadInt32(&dials) != 1 {
		t.Fatalf("got %+v after %d dials", d, dials)
	}
}
