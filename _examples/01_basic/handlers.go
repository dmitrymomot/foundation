package main

import (
	"database/sql"
	"errors"
	"net/http"
	"strings"

	"github.com/dmitrymomot/foundation/_examples/01_basic/db/repository"
	"github.com/dmitrymomot/foundation/core/binder"
	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/sessiontransport"
	"github.com/dmitrymomot/foundation/core/validator"
	"github.com/dmitrymomot/foundation/middleware"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Request/Response types

type SignupRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

type TokenPairResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type UserResponse struct {
	ID    uuid.UUID `json:"id"`
	Name  string    `json:"name"`
	Email string    `json:"email"`
}

type SignupResponse struct {
	User   UserResponse      `json:"user"`
	Tokens TokenPairResponse `json:"tokens"`
}

type LoginResponse struct {
	User   UserResponse      `json:"user"`
	Tokens TokenPairResponse `json:"tokens"`
}

// Auth Handlers

func signupHandler(repo repository.Querier, transport *sessiontransport.JWT[SessionData]) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req SignupRequest
		if err := binder.JSON()(ctx.Request(), &req); err != nil {
			return response.Error(response.ErrBadRequest.WithMessage("Failed to parse request body").WithError(err))
		}

		// Validate input
		rules := []validator.Rule{
			validator.Required("name", req.Name),
			validator.MinLen("name", req.Name, 2),
			validator.Required("email", req.Email),
			validator.ValidEmail("email", req.Email),
			validator.Required("password", req.Password),
			validator.StrongPassword("password", req.Password, validator.DefaultPasswordStrength()),
			validator.NotCommonPassword("password", req.Password),
		}

		if err := validator.Apply(rules...); err != nil {
			validationErrs := validator.ExtractValidationErrors(err)
			return response.Error(
				response.ErrBadRequest.WithDetails(map[string]any{
					"errors": validationErrs,
				}),
			)
		}

		// Hash password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		// Create user
		user, err := repo.CreateUser(ctx.Request().Context(), repository.CreateUserParams{
			Name:         req.Name,
			Email:        strings.ToLower(strings.TrimSpace(req.Email)),
			PasswordHash: passwordHash,
		})
		if err != nil {
			// Check for duplicate email
			if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
				return response.Error(response.ErrConflict.WithMessage("Email already exists"))
			}
			return response.Error(response.ErrInternalServerError)
		}

		// Authenticate session (creates token pair)
		sess, tokens, err := transport.Authenticate(ctx.Request().Context(), ctx.ResponseWriter(), ctx.Request(), user.ID)
		if err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		// Update session data
		sess.Data.Name = user.Name
		sess.Data.Email = user.Email

		return response.JSONWithStatus(SignupResponse{
			User: UserResponse{
				ID:    user.ID,
				Name:  user.Name,
				Email: user.Email,
			},
			Tokens: TokenPairResponse{
				AccessToken:  tokens.AccessToken,
				RefreshToken: tokens.RefreshToken,
				ExpiresIn:    tokens.ExpiresIn,
			},
		}, http.StatusCreated)
	}
}

func loginHandler(repo repository.Querier, transport *sessiontransport.JWT[SessionData]) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req LoginRequest
		if err := binder.JSON()(ctx.Request(), &req); err != nil {
			return response.Error(response.ErrBadRequest.WithError(err))
		}

		// Validate input
		rules := []validator.Rule{
			validator.Required("email", req.Email),
			validator.ValidEmail("email", req.Email),
			validator.Required("password", req.Password),
		}

		if err := validator.Apply(rules...); err != nil {
			validationErrs := validator.ExtractValidationErrors(err)
			return response.Error(
				response.ErrBadRequest.WithDetails(map[string]any{
					"errors": validationErrs,
				}),
			)
		}

		// Find user
		user, err := repo.GetUserByEmail(ctx.Request().Context(), strings.ToLower(strings.TrimSpace(req.Email)))
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return response.Error(response.ErrUnauthorized.WithMessage("Invalid credentials"))
			}
			return response.Error(response.ErrInternalServerError)
		}

		// Verify password
		if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.Password)); err != nil {
			return response.Error(response.ErrUnauthorized.WithMessage("Invalid credentials"))
		}

		// Authenticate session (creates token pair)
		sess, tokens, err := transport.Authenticate(ctx.Request().Context(), ctx.ResponseWriter(), ctx.Request(), user.ID)
		if err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		// Update session data
		sess.Data.Name = user.Name
		sess.Data.Email = user.Email

		return response.JSON(LoginResponse{
			User: UserResponse{
				ID:    user.ID,
				Name:  user.Name,
				Email: user.Email,
			},
			Tokens: TokenPairResponse{
				AccessToken:  tokens.AccessToken,
				RefreshToken: tokens.RefreshToken,
				ExpiresIn:    tokens.ExpiresIn,
			},
		})
	}
}

func refreshHandler(transport *sessiontransport.JWT[SessionData]) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req RefreshRequest
		if err := binder.JSON()(ctx.Request(), &req); err != nil {
			return response.Error(response.ErrBadRequest.WithError(err))
		}

		// Validate input
		if err := validator.Apply(validator.Required("refresh_token", req.RefreshToken)); err != nil {
			return response.Error(response.ErrBadRequest.WithError(err))
		}

		// Refresh tokens
		_, tokens, err := transport.Refresh(ctx.Request().Context(), req.RefreshToken)
		if err != nil {
			return response.Error(response.ErrUnauthorized.WithMessage("Invalid or expired refresh token"))
		}

		return response.JSON(TokenPairResponse{
			AccessToken:  tokens.AccessToken,
			RefreshToken: tokens.RefreshToken,
			ExpiresIn:    tokens.ExpiresIn,
		})
	}
}

func logoutHandler(transport *sessiontransport.JWT[SessionData]) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		// Delete session
		if err := transport.Logout(ctx.Request().Context(), ctx.ResponseWriter(), ctx.Request()); err != nil {
			// Don't fail logout on error, just log it
			// The client should discard their tokens anyway
		}

		return response.JSONWithStatus(map[string]any{
			"message": "Logged out successfully",
		}, http.StatusOK)
	}
}

// Profile Handlers

func getProfileHandler(repo repository.Querier) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		// Get user ID from JWT claims
		claims, ok := middleware.GetStandardClaims(ctx)
		if !ok {
			return response.Error(response.ErrUnauthorized)
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return response.Error(response.ErrUnauthorized)
		}

		// Get user from database
		user, err := repo.GetUserByID(ctx.Request().Context(), userID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return response.Error(response.ErrNotFound.WithMessage("User not found"))
			}
			return response.Error(response.ErrInternalServerError)
		}

		return response.JSON(UserResponse{
			ID:    user.ID,
			Name:  user.Name,
			Email: user.Email,
		})
	}
}

func updatePasswordHandler(repo repository.Querier) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req UpdatePasswordRequest
		if err := binder.JSON()(ctx.Request(), &req); err != nil {
			return response.Error(response.ErrBadRequest.WithError(err))
		}

		// Validate input
		rules := []validator.Rule{
			validator.Required("old_password", req.OldPassword),
			validator.Required("new_password", req.NewPassword),
			validator.StrongPassword("new_password", req.NewPassword, validator.DefaultPasswordStrength()),
			validator.NotCommonPassword("new_password", req.NewPassword),
		}

		if err := validator.Apply(rules...); err != nil {
			validationErrs := validator.ExtractValidationErrors(err)
			return response.Error(
				response.ErrBadRequest.WithDetails(map[string]any{
					"errors": validationErrs,
				}),
			)
		}

		// Get user ID from JWT claims
		claims, ok := middleware.GetStandardClaims(ctx)
		if !ok {
			return response.Error(response.ErrUnauthorized)
		}

		userID, err := uuid.Parse(claims.Subject)
		if err != nil {
			return response.Error(response.ErrUnauthorized)
		}

		// Get user from database
		user, err := repo.GetUserByID(ctx.Request().Context(), userID)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return response.Error(response.ErrNotFound.WithMessage("User not found"))
			}
			return response.Error(response.ErrInternalServerError)
		}

		// Verify old password
		if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.OldPassword)); err != nil {
			return response.Error(response.ErrUnauthorized.WithMessage("Invalid current password"))
		}

		// Hash new password
		newPasswordHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
		if err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		// Update password
		if err := repo.UpdateUserPassword(ctx.Request().Context(), repository.UpdateUserPasswordParams{
			ID:           userID,
			PasswordHash: newPasswordHash,
		}); err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		return response.JSON(map[string]any{
			"message": "Password updated successfully",
		})
	}
}
