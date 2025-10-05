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

// Load session from JWT bearer token.
// Returns ErrNoToken if no bearer token present.
// Creates new anonymous session if token invalid.
func (j *JWT[Data]) Load(ctx handler.Context) (session.Session[Data], error) {
	token := extractBearerToken(ctx.Request())
	if token == "" {
		return session.Session[Data]{}, ErrNoToken
	}

	var claims jwtClaims
	if err := j.signer.Parse(token, &claims); err != nil {
		return session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.JWT(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, j.manager.GetTTL())
	}

	sess, err := j.manager.GetByToken(ctx, claims.ID)
	if err != nil {
		return session.New[Data](session.NewSessionParams{
			Fingerprint: fingerprint.JWT(ctx.Request()),
			IP:          clientip.GetIP(ctx.Request()),
			UserAgent:   ctx.Request().Header.Get("User-Agent"),
		}, j.manager.GetTTL())
	}

	return sess, nil
}

// Save persists session data to the store.
// JWT tokens are immutable, but session data in the database can be updated.
func (j *JWT[Data]) Save(ctx handler.Context, sess session.Session[Data]) error {
	return j.manager.Store(ctx, sess)
}

// Authenticate user. Returns token pair with Session.Token in both JTI and refresh_token.
// Optional data parameter allows setting session data during authentication.
func (j *JWT[Data]) Authenticate(ctx handler.Context, userID uuid.UUID, data ...Data) (session.Session[Data], TokenPair, error) {
	currentSess, err := j.Load(ctx)
	if err != nil && err != ErrNoToken {
		return session.Session[Data]{}, TokenPair{}, err
	}
	// ErrNoToken is acceptable - Load creates anonymous session as fallback

	if err := currentSess.Authenticate(userID, data...); err != nil {
		return session.Session[Data]{}, TokenPair{}, err
	}

	if err := j.manager.Store(ctx, currentSess); err != nil {
		return session.Session[Data]{}, TokenPair{}, err
	}

	pair, err := j.generateTokenPair(currentSess)
	if err != nil {
		return session.Session[Data]{}, TokenPair{}, err
	}

	return currentSess, pair, nil
}

// Logout deletes the session from the store.
// For JWT transport, the client must discard their tokens.
func (j *JWT[Data]) Logout(ctx handler.Context) error {
	sess, err := j.Load(ctx)
	if err != nil {
		if err == ErrNoToken {
			return nil
		}
		return err
	}

	sess.Logout() // Mark for deletion

	// Store will delete the session
	if err := j.manager.Store(ctx, sess); err != nil && !errors.Is(err, session.ErrNotAuthenticated) {
		return err
	}

	return nil
}

// Delete session from store.
func (j *JWT[Data]) Delete(ctx handler.Context) error {
	sess, err := j.Load(ctx)
	if err != nil {
		if err == ErrNoToken {
			return nil
		}
		return err
	}

	sess.Logout() // Mark for deletion

	// Store will delete the session
	if err := j.manager.Store(ctx, sess); err != nil && !errors.Is(err, session.ErrNotAuthenticated) {
		return err
	}

	return nil
}

// Store persists session state.
func (j *JWT[Data]) Store(ctx handler.Context, sess session.Session[Data]) error {
	return j.manager.Store(ctx, sess)
}

// Auth generates a new token pair after authentication.
// Call this after sess.Authenticate() to get JWT tokens.
func (j *JWT[Data]) Auth(sess session.Session[Data]) (TokenPair, error) {
	if !sess.IsAuthenticated() {
		return TokenPair{}, session.ErrNotAuthenticated
	}

	return j.generateTokenPair(sess)
}

// Refresh validates the refresh token and generates a new token pair.
// This rotates the session token in the database for security.
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

	// Rotate session token for security
	if err := sess.Refresh(); err != nil {
		return TokenPair{}, err
	}

	// Save rotated session
	if err := j.manager.Store(ctx, sess); err != nil {
		return TokenPair{}, err
	}

	// Generate new token pair
	return j.Auth(sess)
}

// generateTokenPair creates access and refresh token pair.
func (j *JWT[Data]) generateTokenPair(sess session.Session[Data]) (TokenPair, error) {
	now := time.Now()
	expiresAt := now.Add(j.accessTTL)

	accessClaims := jwtClaims{
		StandardClaims: jwt.StandardClaims{
			ID:        sess.Token, // Session.Token in JTI
			Subject:   sess.UserID.String(),
			Issuer:    j.issuer,
			IssuedAt:  now.Unix(),
			ExpiresAt: expiresAt.Unix(),
		},
	}

	// Refresh token uses same JTI but session expiration (longer-lived)
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
