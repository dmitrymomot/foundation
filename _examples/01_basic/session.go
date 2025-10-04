package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"

	"github.com/dmitrymomot/foundation/_examples/01_basic/db/repository"
	"github.com/dmitrymomot/foundation/core/session"
	"github.com/google/uuid"
)

type SessionData struct {
	Name  string
	Email string
}

type sessionStorage struct {
	repo repository.Querier
}

// fingerprintToDeviceID converts fingerprint to deterministic UUID
func fingerprintToDeviceID(fingerprint string) uuid.UUID {
	hash := sha256.Sum256([]byte(fingerprint))
	deviceID, _ := uuid.FromBytes(hash[:16])
	return deviceID
}

// GetByID retrieves a session by its ID
func (s *sessionStorage) GetByID(ctx context.Context, id uuid.UUID) (*session.Session[SessionData], error) {
	dbSession, err := s.repo.GetSessionByID(ctx, id)
	if err != nil {
		return nil, session.ErrNotFound
	}

	// Deserialize JSONB data
	var data SessionData
	if err := json.Unmarshal(dbSession.Data, &data); err != nil {
		return nil, err
	}

	// Convert database model to session.Session
	sess := &session.Session[SessionData]{
		ID:          dbSession.ID,
		Token:       dbSession.Token,
		UserID:      uuid.Nil,
		Fingerprint: dbSession.Fingerprint,
		IP:          dbSession.IpAddress,
		Data:        data,
		ExpiresAt:   dbSession.ExpiresAt,
		CreatedAt:   dbSession.CreatedAt,
		UpdatedAt:   dbSession.UpdatedAt,
	}

	// Handle nullable user_id
	if dbSession.UserID != nil {
		sess.UserID = *dbSession.UserID
	}

	// Handle nullable user_agent
	if dbSession.UserAgent != nil {
		sess.UserAgent = *dbSession.UserAgent
	}

	return sess, nil
}

// GetByToken retrieves a session by its token
func (s *sessionStorage) GetByToken(ctx context.Context, token string) (*session.Session[SessionData], error) {
	dbSession, err := s.repo.GetSessionByToken(ctx, token)
	if err != nil {
		return nil, session.ErrNotFound
	}

	// Deserialize JSONB data
	var data SessionData
	if err := json.Unmarshal(dbSession.Data, &data); err != nil {
		return nil, err
	}

	// Convert database model to session.Session
	sess := &session.Session[SessionData]{
		ID:          dbSession.ID,
		Token:       dbSession.Token,
		UserID:      uuid.Nil,
		Fingerprint: dbSession.Fingerprint,
		IP:          dbSession.IpAddress,
		Data:        data,
		ExpiresAt:   dbSession.ExpiresAt,
		CreatedAt:   dbSession.CreatedAt,
		UpdatedAt:   dbSession.UpdatedAt,
	}

	// Handle nullable user_id
	if dbSession.UserID != nil {
		sess.UserID = *dbSession.UserID
	}

	// Handle nullable user_agent
	if dbSession.UserAgent != nil {
		sess.UserAgent = *dbSession.UserAgent
	}

	return sess, nil
}

// Save stores or updates a session
func (s *sessionStorage) Save(ctx context.Context, sess *session.Session[SessionData]) error {
	// Convert fingerprint to device_id (use stable UUID if no fingerprint)
	deviceID := uuid.New()
	if sess.Fingerprint != "" {
		deviceID = fingerprintToDeviceID(sess.Fingerprint)
	}

	// Serialize session data to JSONB
	dataJSON, err := json.Marshal(sess.Data)
	if err != nil {
		return err
	}

	// Prepare nullable user_id
	var userID *uuid.UUID
	if sess.UserID != uuid.Nil {
		userID = &sess.UserID
	}

	// Prepare nullable user_agent
	var userAgent *string
	if sess.UserAgent != "" {
		userAgent = &sess.UserAgent
	}

	// Upsert into database
	_, err = s.repo.UpsertSession(ctx, repository.UpsertSessionParams{
		ID:          sess.ID,
		Token:       sess.Token,
		DeviceID:    deviceID,
		Fingerprint: sess.Fingerprint,
		IpAddress:   sess.IP,
		UserAgent:   userAgent,
		UserID:      userID,
		Data:        dataJSON,
		ExpiresAt:   sess.ExpiresAt,
		CreatedAt:   sess.CreatedAt,
		UpdatedAt:   sess.UpdatedAt,
	})

	return err
}

// Delete removes a session by its ID
func (s *sessionStorage) Delete(ctx context.Context, id uuid.UUID) error {
	return s.repo.DeleteSessionByID(ctx, id)
}

// DeleteExpired removes all expired sessions
func (s *sessionStorage) DeleteExpired(ctx context.Context) error {
	return s.repo.DeleteExpiredSessions(ctx)
}

// Ensure sessionStorage implements session.Store interface
var _ session.Store[SessionData] = (*sessionStorage)(nil)
