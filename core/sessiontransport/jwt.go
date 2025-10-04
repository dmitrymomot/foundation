package sessiontransport

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/dmitrymomot/foundation/pkg/fingerprint"
	"github.com/dmitrymomot/foundation/pkg/jwt"
)

// TokenPair represents JWT access and refresh tokens.
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
		return j.manager.New(ctx, fingerprint.JWT(ctx.Request()))
	}

	// JTI claim contains Session.Token
	sess, err := j.manager.GetByToken(ctx, claims.ID)
	if err != nil {
		return j.manager.New(ctx, fingerprint.JWT(ctx.Request()))
	}

	return sess, nil
}

// Save is no-op for JWT (tokens are immutable).
func (j *JWT[Data]) Save(ctx handler.Context, sess session.Session[Data]) error {
	// JWT tokens are immutable - session changes are persisted to store only
	return nil
}

// Authenticate user. Returns token pair with Session.Token in both JTI and refresh_token.
func (j *JWT[Data]) Authenticate(ctx handler.Context, userID uuid.UUID) (session.Session[Data], TokenPair, error) {
	currentSess, err := j.Load(ctx)
	if err != nil && err != ErrNoToken {
		return session.Session[Data]{}, TokenPair{}, err
	}
	// If ErrNoToken, currentSess will be anonymous from Load fallback

	// Rotates token for security
	authSess, err := j.manager.Authenticate(ctx, currentSess, userID)
	if err != nil {
		return session.Session[Data]{}, TokenPair{}, err
	}

	pair, err := j.generateTokenPair(authSess)
	if err != nil {
		return session.Session[Data]{}, TokenPair{}, err
	}

	return authSess, pair, nil
}

// Refresh rotates refresh token and generates new access token.
func (j *JWT[Data]) Refresh(ctx context.Context, refreshToken string) (session.Session[Data], TokenPair, error) {
	var claims jwtClaims
	if err := j.signer.Parse(refreshToken, &claims); err != nil {
		return session.Session[Data]{}, TokenPair{}, ErrInvalidToken
	}

	// JTI contains Session.Token
	sess, err := j.manager.GetByToken(ctx, claims.ID)
	if err != nil {
		return session.Session[Data]{}, TokenPair{}, err
	}

	// Refresh rotates token while keeping same session ID (critical for audit logs)
	refreshedSess, err := j.manager.Refresh(ctx, sess)
	if err != nil {
		return session.Session[Data]{}, TokenPair{}, err
	}

	pair, err := j.generateTokenPair(refreshedSess)
	if err != nil {
		return session.Session[Data]{}, TokenPair{}, err
	}

	return refreshedSess, pair, nil
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

	return j.manager.Delete(ctx, sess.ID)
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

	return j.manager.Delete(ctx, sess.ID)
}

// Touch updates session expiration in the database if the touch interval has elapsed.
// This is called by the session middleware after each request to extend session lifetime.
//
// For JWT transport, tokens are immutable so we only update the database record.
// The method retrieves the session from the store, which automatically touches it if needed.
// The client continues using their existing JWT until it expires or they refresh.
func (j *JWT[Data]) Touch(ctx handler.Context, sess session.Session[Data]) error {
	// JWT is immutable - retrieve session to trigger database touch if needed
	_, err := j.manager.GetByID(ctx, sess.ID)
	return err
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
