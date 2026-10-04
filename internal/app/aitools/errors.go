package aitools

import (
	"context"
	"errors"
	"net"
	"net/url"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/warmbly/warmbly/internal/errx"
)

var (
	// ErrToolNotFound is returned by Registry.Call for an unknown tool name.
	ErrToolNotFound = errors.New("tool not found")
	// ErrToolForbidden is returned when the invocation lacks the tool's
	// required permission.
	ErrToolForbidden = errors.New("tool not permitted")
	// ErrInvalidArgs is returned by a handler when the model's JSON arguments
	// fail to decode or validate. It is fed back to the model so it can retry.
	ErrInvalidArgs = errors.New("invalid tool arguments")
	// errUniboxNotEntitled mirrors the HTTP unibox 403 when the org has no
	// active trial/paid plan.
	errUniboxNotEntitled = errors.New("the unified inbox requires an active trial or paid subscription")
)

// errRecipientSuppressed reports a refused send to a suppressed recipient
// (bounced/complained/unsubscribed), mirroring the compose handler's 400. The
// message is fed back to the model so it can explain why it did not send.
func errRecipientSuppressed(msg string) error {
	return errors.New(msg)
}

// PublicMessage is the text an external caller may read for a tool failure;
// ok is false for a server-side failure, whose text belongs in the log only.
func PublicMessage(err error) (msg string, ok bool) {
	var xe *errx.Error
	if errors.As(err, &xe) {
		if xe.Code == errx.Internal && !xe.Public {
			return "", false
		}
		return xe.Message, true
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return "not found", true
	}
	var pgErr *pgconn.PgError
	var connErr *pgconn.ConnectError
	var netErr net.Error
	var urlErr *url.Error
	if errors.As(err, &pgErr) || errors.As(err, &connErr) || errors.As(err, &netErr) || errors.As(err, &urlErr) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "", false
	}
	return err.Error(), true
}
