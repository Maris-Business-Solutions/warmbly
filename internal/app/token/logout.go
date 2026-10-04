package token

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/infrastructure/db"
)

func (s *tokenService) RevokeSession(ctx context.Context, accessToken string) *errx.Error {
	sess, err := s.ValidateAccessToken(ctx, accessToken)
	if err != nil {
		return err
	}

	now := time.Now()

	tx, xerr := s.db.Begin(ctx)
	if xerr != nil {
		db.CaptureError(xerr, "", nil, "begin")
		return errx.InternalError()
	}
	defer tx.Rollback(ctx)

	if err := s.tokenRepository.RevokeSession(ctx, tx, sess.ID, now); err != nil {
		return err
	}

	if err := tx.Commit(ctx); err != nil {
		db.CaptureError(err, "", nil, "commit")
		return errx.InternalError()
	}

	// Evicted after the commit, so a concurrent read cannot re-cache the live row.
	if err := s.evictRevoked(ctx, sess.ID, sess.UserID, now); err != nil {
		return err
	}
	s.notifyRevoked(ctx, sess.UserID)

	return nil
}

// RevokeAllSession ends every session the caller's user holds, this one included.
func (s *tokenService) RevokeAllSession(ctx context.Context, accessToken string) *errx.Error {
	sess, err := s.ValidateAccessToken(ctx, accessToken)
	if err != nil {
		return err
	}
	return s.RevokeOtherSessions(ctx, sess.UserID, uuid.Nil)
}
