package main

import (
	"net/http"

	"github.com/dmitrymomot/foundation/_examples/01_basic/db/repository"
	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/validator"
	"github.com/dmitrymomot/foundation/integration/database/pg"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Request/Response types

type SignupRequest struct {
	Name     string `json:"name" sanitize:"trim,title" validate:"required;min:2"`
	Email    string `json:"email" sanitize:"email" validate:"required;email"`
	Password string `json:"password" validate:"required;min:8;strong_password;not_common_password"`
}

type LoginRequest struct {
	Email    string `json:"email" sanitize:"email" validate:"required;email"`
	Password string `json:"password" validate:"required"`
}

type RefreshRequest struct {
	RefreshToken string `json:"refresh_token" sanitize:"trim" validate:"required"`
}

type UpdatePasswordRequest struct {
	OldPassword string `json:"old_password" validate:"required"`
	NewPassword string `json:"new_password" validate:"required;min:8;strong_password;not_common_password"`
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

func signupHandler(repo repository.Querier) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req SignupRequest
		if err := ctx.Bind(&req); err != nil {
			if validator.IsValidationError(err) {
				validationErrs := validator.ExtractValidationErrors(err)
				return response.Error(
					response.ErrBadRequest.WithDetails(map[string]any{
						"errors": validationErrs,
					}),
				)
			}
			return response.Error(response.ErrBadRequest.WithMessage("Failed to parse request").WithError(err))
		}

		// Hash password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		// Create user (name and email already sanitized by Bind)
		user, err := repo.CreateUser(ctx, repository.CreateUserParams{
			Name:         req.Name,
			Email:        req.Email,
			PasswordHash: passwordHash,
		})
		if err != nil {
			// Check for duplicate email (unique constraint violation)
			if pg.IsDuplicateKeyError(err) {
				return response.Error(response.ErrConflict.WithMessage("Email already exists"))
			}
			return response.Error(response.ErrInternalServerError)
		}

		// Authenticate session (creates token pair)
		tokens, err := ctx.Auth(user.ID)
		if err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		// Get session and update session data
		sess, _ := ctx.Session()
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

func loginHandler(repo repository.Querier) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req LoginRequest
		if err := ctx.Bind(&req); err != nil {
			if validator.IsValidationError(err) {
				validationErrs := validator.ExtractValidationErrors(err)
				return response.Error(
					response.ErrBadRequest.WithDetails(map[string]any{
						"errors": validationErrs,
					}),
				)
			}
			return response.Error(response.ErrBadRequest.WithMessage("Failed to parse request").WithError(err))
		}

		// Find user (email already sanitized by Bind)
		user, err := repo.GetUserByEmail(ctx, req.Email)
		if err != nil {
			if pg.IsNotFoundError(err) {
				return response.Error(response.ErrUnauthorized.WithMessage("Invalid credentials"))
			}
			return response.Error(response.ErrInternalServerError)
		}

		// Verify password
		if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.Password)); err != nil {
			return response.Error(response.ErrUnauthorized.WithMessage("Invalid credentials"))
		}

		// Authenticate session (creates token pair)
		tokens, err := ctx.Auth(user.ID)
		if err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		// Get session and update session data
		sess, _ := ctx.Session()
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

func refreshHandler() handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req RefreshRequest
		if err := ctx.Bind(&req); err != nil {
			if validator.IsValidationError(err) {
				validationErrs := validator.ExtractValidationErrors(err)
				return response.Error(
					response.ErrBadRequest.WithDetails(map[string]any{
						"errors": validationErrs,
					}),
				)
			}
			return response.Error(response.ErrBadRequest.WithMessage("Failed to parse request").WithError(err))
		}

		// Refresh tokens
		tokens, err := ctx.Refresh(req.RefreshToken)
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

func logoutHandler() handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		// Delete session
		if err := ctx.Logout(); err != nil {
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
		// Get user from database
		user, err := repo.GetUserByID(ctx, ctx.UserID())
		if err != nil {
			if pg.IsNotFoundError(err) {
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
		if err := ctx.Bind(&req); err != nil {
			if validator.IsValidationError(err) {
				validationErrs := validator.ExtractValidationErrors(err)
				return response.Error(
					response.ErrBadRequest.WithDetails(map[string]any{
						"errors": validationErrs,
					}),
				)
			}
			return response.Error(response.ErrBadRequest.WithMessage("Failed to parse request").WithError(err))
		}

		// Get user from database
		user, err := repo.GetUserByID(ctx, ctx.UserID())
		if err != nil {
			if pg.IsNotFoundError(err) {
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
		if err := repo.UpdateUserPassword(ctx, repository.UpdateUserPasswordParams{
			ID:           ctx.UserID(),
			PasswordHash: newPasswordHash,
		}); err != nil {
			return response.Error(response.ErrInternalServerError)
		}

		return response.JSON(map[string]any{
			"message": "Password updated successfully",
		})
	}
}
