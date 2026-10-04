package handler

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/warmbly/warmbly/internal/api/middleware"
	"github.com/warmbly/warmbly/internal/app/token"
	"github.com/warmbly/warmbly/internal/errx"
	"github.com/warmbly/warmbly/internal/models"
	"github.com/warmbly/warmbly/internal/pkg/argon2"
)

type reauthRequest struct {
	// Password is the account password. Optional for an account that has none
	// (passkey or SSO only), which confirms with Code instead.
	Password string `json:"password"`
	// Code is a current TOTP or recovery code.
	Code string `json:"code"`
}

type reauthResponse struct {
	// ValidForSeconds is how long the confirmation lasts, so the client can
	// decide whether to ask again rather than letting the next call fail.
	ValidForSeconds int `json:"valid_for_seconds"`
}

// Reauth re-proves the account holder behind a live session.
//
// Sensitive changes (minting an API key, registering a passkey, transferring a
// workspace, scheduling a deletion) require a recent confirmation; this is
// where it is given. Either factor is accepted: an account with no password
// confirms with its authenticator, and an account with no 2FA confirms with its
// password.
func (h *Handler) Reauth(c *gin.Context) {
	session := middleware.GetSession(c)
	if session == nil {
		errx.Handle(c, errx.ErrUnauthorized)
		return
	}
	uid, err := uuid.Parse(middleware.GetUserID(c))
	if err != nil {
		errx.Handle(c, errx.ErrUnauthorized)
		return
	}

	var req reauthRequest
	if berr := c.ShouldBindJSON(&req); berr != nil {
		errx.Handle(c, errx.InvalidBody(berr))
		return
	}

	// An account created through Google, Apple or SSO has no password, and
	// until it enrols 2FA it has no second factor either. There is nothing for
	// it to confirm with, and silently accepting the live session instead would
	// turn a stolen token into a permanent API key. Say what to do rather than
	// refusing a credential the account does not have.
	if !h.hasReauthFactor(c, uid) {
		errx.Handle(c, errReauthNoFactor())
		return
	}

	if req.Password == "" && req.Code == "" {
		errx.Handle(c, errx.New(errx.BadRequest, "provide your password or a two-factor code"))
		return
	}

	// This endpoint checks a password, so it is a second place to guess one.
	// Refused before the hash comparison, so a caller past the budget cannot
	// measure Argon2's timing either.
	ctx := c.Request.Context()
	if !h.AuthService.ReserveReauthAttempt(ctx, uid) {
		errx.Handle(c, errx.ErrAuthLimit)
		return
	}

	// The reserved attempt stays charged on a miss.
	if !h.reauthProofValid(c, uid, req) {
		// One message for both factors: which one matched is not something an
		// attacker holding a token should learn here.
		errx.Handle(c, errx.ErrCredentials)
		return
	}
	h.AuthService.ClearReauthFailures(ctx, uid)

	if xerr := h.TokenService.StampReauth(c.Request.Context(), session.ID); xerr != nil {
		errx.Handle(c, xerr)
		return
	}

	c.JSON(http.StatusOK, reauthResponse{ValidForSeconds: int(token.ReauthWindow.Seconds())})
}

// errReauthNoFactor answers an account with nothing to confirm with. Its way to
// a factor is enrolling 2FA, which a fresh sign-in unlocks.
func errReauthNoFactor() *errx.Error {
	return errx.NewWithIdentifier(errx.BadRequest, "reauth_no_factor",
		"This account has no password and no two-factor authentication, so there is nothing to confirm with. "+
			"Sign out and sign in again, then turn on two-factor authentication under Settings > Security within five minutes of signing in.")
}

// allowEnrollment gates adding an authenticator, which /auth/reauth then
// accepts. See enrollmentGate.
func (h *Handler) allowEnrollment(c *gin.Context, uid uuid.UUID) bool {
	session := middleware.GetSession(c)
	if session == nil {
		errx.JSON(c, errx.ErrUnauthorized)
		return false
	}
	if xerr := enrollmentGate(session, time.Now(), func() bool { return h.hasReauthFactor(c, uid) }); xerr != nil {
		errx.JSON(c, xerr)
		return false
	}
	return true
}

// enrollmentGate passes a recent reauth; otherwise an account with a factor
// must confirm, and one with none passes only on a session signed in within
// the reauth window, since a fresh sign-in is the only proof it can give.
func enrollmentGate(session *models.Session, now time.Time, hasFactor func() bool) *errx.Error {
	if session.ReauthAt != nil && now.Sub(*session.ReauthAt) <= token.ReauthWindow {
		return nil
	}
	if hasFactor() {
		return errx.NewWithIdentifier(errx.Forbidden, "reauth_required",
			"Confirm it is you before making this change.")
	}
	if now.Sub(session.CreatedAt) <= token.ReauthWindow {
		return nil
	}
	return errReauthNoFactor()
}

// hasReauthFactor reports whether the account has anything to re-authenticate
// with: a stored password, or an enrolled authenticator.
func (h *Handler) hasReauthFactor(c *gin.Context, uid uuid.UUID) bool {
	ctx := c.Request.Context()

	if h.TwoFAService != nil {
		if enabled, err := h.TwoFAService.IsEnabled(ctx, uid); err == nil && enabled {
			return true
		}
	}
	hash, xerr := h.AuthService.PasswordHashFor(ctx, uid)
	return xerr == nil && hash != ""
}

// reauthProofValid checks whichever factor the caller offered.
func (h *Handler) reauthProofValid(c *gin.Context, uid uuid.UUID, req reauthRequest) bool {
	ctx := c.Request.Context()

	if req.Code != "" && h.TwoFAService != nil {
		if h.TwoFAService.VerifyCurrentCode(ctx, uid, req.Code) {
			return true
		}
	}

	if req.Password != "" {
		hash, xerr := h.AuthService.PasswordHashFor(ctx, uid)
		if xerr != nil || hash == "" {
			return false
		}
		ok, verr := argon2.Verify(req.Password, hash)
		return verr == nil && ok
	}

	return false
}
