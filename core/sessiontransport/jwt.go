package sessiontransport

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/pkg/clientip"
	"github.com/dmitrymomot/foundation/pkg/fingerprint"
	"github.com/dmitrymomot/foundation/pkg/jwt"
)

// TokenPair contains JWT access and refresh tokens with metadata.
type TokenPair struct {
	AccessToken  string    `json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	TokenType    string    `json:"token_type"`
	ExpiresIn    int       `json:"expires_in"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// JWT provides JWT-based session transport.
// It stores Session.Token in JTI claim (unified approach with Cookie).
type JWT[Data any] struct {
	manager   *session.Manager[Data]
	signer    *jwt.Service
	accessTTL time.Duration
	issuer    string
}

// jwtClaims represents JWT custom claims for session tokens.
type jwtClaims struct {
	jwt.StandardClaims
}

// NewJWT creates a new JWT-based session transport.
func NewJWT[Data any](mgr *session.Manager[Data], secretKey string, accessTTL time.Duration, issuer string) (*JWT[Data], error) {
	signer, err := jwt.NewFromString(secretKey)
	if err != nil {
		return nil, err
	}

	return &JWT[Data]{
		manager:   mgr,
		signer:    signer,
		accessTTL: accessTTL,
		issuer:    issuer,
	}, nil
}

// Load extracts and validates session from JWT bearer token.
// Returns ErrNoToken if no bearer token present, creates anonymous session if token invalid.
func (j *JWT[Data]) Load(ctx handler.Context) (*session.Session[Data], error) {
	token := extractBearerToken(ctx.Request())
	if token == "" {
		return nil, ErrNoToken
	}

	var claims jwtClaims
	if err := j.signer.Parse(token, &claims); err != nil {
		sess, newErr := session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.JWT(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, j.manager.GetTTL())
		if newErr != nil {
			return nil, newErr
		}
		return &sess, nil
	}

	sess, err := j.manager.GetByToken(ctx, claims.ID)
	if err != nil {
		sess, newErr := session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.JWT(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, j.manager.GetTTL())
		if newErr != nil {
			return nil, newErr
		}
		return &sess, nil
	}

	return sess, nil
}

// Save persists session data to the store.
// Note: JWT tokens are immutable; only database session data is updated.
func (j *JWT[Data]) Save(ctx handler.Context, sess *session.Session[Data]) error {
	return j.manager.Store(ctx, sess)
}

// Authenticate creates an authenticated session and returns a token pair.
// Session.Token is embedded in both access and refresh token JTI claims.
// Optional data parameter sets session data during authentication.
func (j *JWT[Data]) Authenticate(ctx handler.Context, userID uuid.UUID, data ...Data) (*session.Session[Data], TokenPair, error) {
	currentSess, err := j.Load(ctx)
	if err != nil {
		if err != ErrNoToken {
			return nil, TokenPair{}, err
		}
		// ErrNoToken - create new anonymous session as fallback
		sess, newErr := session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.JWT(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, j.manager.GetTTL())
		if newErr != nil {
			return nil, TokenPair{}, newErr
		}
		currentSess = &sess
	}

	if err := currentSess.Authenticate(userID, data...); err != nil {
		return nil, TokenPair{}, err
	}

	if err := j.manager.Store(ctx, currentSess); err != nil {
		return nil, TokenPair{}, err
	}

	pair, err := j.generateTokenPair(currentSess)
	if err != nil {
		return nil, TokenPair{}, err
	}

	return currentSess, pair, nil
}

// Logout deletes the session from the store.
// Client must discard JWT tokens as they cannot be invalidated server-side.
func (j *JWT[Data]) Logout(ctx handler.Context) error {
	sess, err := j.Load(ctx)
	if err != nil {
		if err == ErrNoToken {
			return nil
		}
		return err
	}

	sess.Logout()

	if err := j.manager.Store(ctx, sess); err != nil && !errors.Is(err, session.ErrNotAuthenticated) {
		return err
	}

	return nil
}

// Delete removes the session from store.
func (j *JWT[Data]) Delete(ctx handler.Context) error {
	sess, err := j.Load(ctx)
	if err != nil {
		if err == ErrNoToken {
			return nil
		}
		return err
	}

	sess.Logout()

	if err := j.manager.Store(ctx, sess); err != nil && !errors.Is(err, session.ErrNotAuthenticated) {
		return err
	}

	return nil
}

// Store persists session state.
func (j *JWT[Data]) Store(ctx handler.Context, sess *session.Session[Data]) error {
	return j.manager.Store(ctx, sess)
}

// Auth generates a new token pair after authentication.
// Call this after sess.Authenticate() to get JWT tokens.
func (j *JWT[Data]) Auth(sess *session.Session[Data]) (TokenPair, error) {
	if !sess.IsAuthenticated() {
		return TokenPair{}, session.ErrNotAuthenticated
	}

	return j.generateTokenPair(sess)
}

// Refresh validates the refresh token and generates a new token pair.
// Rotates the session token in database for security.
func (j *JWT[Data]) Refresh(ctx context.Context, refreshToken string) (TokenPair, error) {
	var claims jwtClaims
	if err := j.signer.Parse(refreshToken, &claims); err != nil {
		return TokenPair{}, ErrInvalidToken
	}

	sess, err := j.manager.GetByToken(ctx, claims.ID)
	if err != nil {
		return TokenPair{}, err
	}

	if !sess.IsAuthenticated() {
		return TokenPair{}, session.ErrNotAuthenticated
	}

	if err := sess.Refresh(); err != nil {
		return TokenPair{}, err
	}

	if err := j.manager.Store(ctx, sess); err != nil {
		return TokenPair{}, err
	}

	return j.Auth(sess)
}

// generateTokenPair creates access and refresh token pair.
func (j *JWT[Data]) generateTokenPair(sess *session.Session[Data]) (TokenPair, error) {
	now := time.Now()
	expiresAt := now.Add(j.accessTTL)

	accessClaims := jwtClaims{
		StandardClaims: jwt.StandardClaims{
			ID:        sess.Token,
			Subject:   sess.UserID.String(),
			Issuer:    j.issuer,
			IssuedAt:  now.Unix(),
			ExpiresAt: expiresAt.Unix(),
		},
	}

	// Refresh token shares same JTI but uses longer session expiration
	refreshClaims := jwtClaims{
		StandardClaims: jwt.StandardClaims{
			ID:        sess.Token,
			Subject:   sess.UserID.String(),
			Issuer:    j.issuer,
			IssuedAt:  now.Unix(),
			ExpiresAt: sess.ExpiresAt.Unix(),
		},
	}

	accessToken, err := j.signer.Generate(accessClaims)
	if err != nil {
		return TokenPair{}, err
	}

	refreshToken, err := j.signer.Generate(refreshClaims)
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		TokenType:    "Bearer",
		ExpiresIn:    int(j.accessTTL.Seconds()),
		ExpiresAt:    expiresAt,
	}, nil
}

// extractBearerToken extracts Bearer token from Authorization header.
func extractBearerToken(r *http.Request) string {
	authHeader := r.Header.Get("Authorization")
	if authHeader == "" {
		return ""
	}

	parts := strings.SplitN(authHeader, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}

	return parts[1]
}
