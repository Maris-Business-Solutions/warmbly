package integration

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"reflect"

	"github.com/jackc/pgx/v5"
)

// lowerLayerError marks a failure below the service (keys, storage), never shown to a caller.
type lowerLayerError struct{ err error }

func (e lowerLayerError) Error() string { return e.err.Error() }
func (e lowerLayerError) Unwrap() error { return e.err }

func lowerLayer(err error) error {
	if err == nil {
		return nil
	}
	return lowerLayerError{err}
}

// publicSentinels answer with their own text, whatever else the error carries.
var publicSentinels = []error{
	ErrUseOAuth, ErrOAuthNotConfigured, ErrPushUnsupported, ErrPushReauth,
	ErrNotInboundProvider, ErrInboundSigningKeyLength, ErrInboundAutomationNotFound,
}

// lowerSentinels are plain-text errors from the standard library and drivers.
var lowerSentinels = []error{
	sql.ErrNoRows, pgx.ErrNoRows, context.Canceled, context.DeadlineExceeded, io.EOF, io.ErrUnexpectedEOF,
}

var (
	plainErrorType = reflect.TypeOf(errors.New(""))
	wrapErrorType  = reflect.TypeOf(fmt.Errorf("%w", errors.New("")))
)

// PublicMessage returns the text of one of the service's own refusals: a
// sentinel above, or a chain of plain messages the service wrote. Anything
// else came from a lower layer and is not for the caller.
func PublicMessage(err error) (string, bool) {
	if err == nil {
		return "", false
	}
	for _, s := range publicSentinels {
		if errors.Is(err, s) {
			return s.Error(), true
		}
	}
	for _, s := range lowerSentinels {
		if errors.Is(err, s) {
			return "", false
		}
	}
	for e := err; e != nil; e = errors.Unwrap(e) {
		switch t := reflect.TypeOf(e); {
		case t == wrapErrorType:
			continue
		case t == plainErrorType:
			return err.Error(), true
		}
		// A URL the caller typed that does not parse.
		var ue *url.Error
		if errors.As(e, &ue) && ue.Op == "parse" && reflect.TypeOf(e) == reflect.TypeOf(ue) {
			return err.Error(), true
		}
		return "", false
	}
	return "", false
}
