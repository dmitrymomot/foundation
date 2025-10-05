package main

import (
	"html/template"

	"github.com/dmitrymomot/foundation/_examples/web/db/repository"
	"github.com/dmitrymomot/foundation/core/handler"
	"github.com/dmitrymomot/foundation/core/response"
	"github.com/dmitrymomot/foundation/core/validator"
	"github.com/dmitrymomot/foundation/integration/database/pg"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Request types for form binding

type SignupRequest struct {
	Name     string `form:"name" sanitize:"trim,title" validate:"required;min:2"`
	Email    string `form:"email" sanitize:"email" validate:"required;email"`
	Password string `form:"password" validate:"required;min:8;strong_password;not_common_password"`
}

type LoginRequest struct {
	Email    string `form:"email" sanitize:"email" validate:"required;email"`
	Password string `form:"password" validate:"required"`
}

// Template data structures

type PageData struct {
	Title string
	Error string
}

type SignupPageData struct {
	Title string
	Error string
}

type LoginPageData struct {
	Title string
	Error string
}

type ProfilePageData struct {
	Title   string
	User    UserData
	Session SessionInfo
}

type UserData struct {
	ID    uuid.UUID
	Name  string
	Email string
}

type SessionInfo struct {
	ID        uuid.UUID
	IP        string
	UserAgent string
	CreatedAt string
	ExpiresAt string
}

// Page handlers (GET - render forms)

func signupPageHandler(tmpl *template.Template) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		data := SignupPageData{
			Title: "Sign Up",
		}
		return response.Template(tmpl, data)
	}
}

func loginPageHandler(tmpl *template.Template) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		data := LoginPageData{
			Title: "Log In",
		}
		return response.Template(tmpl, data)
	}
}

// Form submission handlers (POST)

func signupHandler(repo repository.Querier, tmpl *template.Template) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req SignupRequest
		if err := ctx.Bind(&req); err != nil {
			errorMsg := "Invalid form data"
			if validationErrs := validator.ExtractValidationErrors(err); len(validationErrs) > 0 {
				errorMsg = validationErrs[0].Message
			}
			return response.Template(tmpl, SignupPageData{
				Title: "Sign Up",
				Error: errorMsg,
			})
		}

		// Hash password
		passwordHash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return response.Template(tmpl, SignupPageData{
				Title: "Sign Up",
				Error: "Failed to process password",
			})
		}

		// Create user
		user, err := repo.CreateUser(ctx, repository.CreateUserParams{
			Name:         req.Name,
			Email:        req.Email,
			PasswordHash: passwordHash,
		})
		if err != nil {
			errorMsg := "Failed to create account"
			if pg.IsDuplicateKeyError(err) {
				errorMsg = "Email already exists"
			}
			return response.Template(tmpl, SignupPageData{
				Title: "Sign Up",
				Error: errorMsg,
			})
		}

		// Authenticate with session data (creates session cookie)
		if err := ctx.Auth(user.ID, SessionData{
			Name:  user.Name,
			Email: user.Email,
		}); err != nil {
			return response.Template(tmpl, SignupPageData{
				Title: "Sign Up",
				Error: "Failed to create session",
			})
		}

		// Redirect to profile using POST-redirect-GET pattern
		return response.RedirectSeeOther("/")
	}
}

func loginHandler(repo repository.Querier, tmpl *template.Template) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		var req LoginRequest
		if err := ctx.Bind(&req); err != nil {
			errorMsg := "Invalid form data"
			if validationErrs := validator.ExtractValidationErrors(err); len(validationErrs) > 0 {
				errorMsg = validationErrs[0].Message
			}
			return response.Template(tmpl, LoginPageData{
				Title: "Log In",
				Error: errorMsg,
			})
		}

		// Find user
		user, err := repo.GetUserByEmail(ctx, req.Email)
		if err != nil {
			return response.Template(tmpl, LoginPageData{
				Title: "Log In",
				Error: "Invalid email or password",
			})
		}

		// Verify password
		if err := bcrypt.CompareHashAndPassword(user.PasswordHash, []byte(req.Password)); err != nil {
			return response.Template(tmpl, LoginPageData{
				Title: "Log In",
				Error: "Invalid email or password",
			})
		}

		// Authenticate with session data (creates session cookie)
		if err := ctx.Auth(user.ID, SessionData{
			Name:  user.Name,
			Email: user.Email,
		}); err != nil {
			return response.Template(tmpl, LoginPageData{
				Title: "Log In",
				Error: "Failed to create session",
			})
		}

		// Redirect to profile using POST-redirect-GET pattern
		return response.RedirectSeeOther("/")
	}
}

func profileHandler(repo repository.Querier, tmpl *template.Template) handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		// Get user from database
		user, err := repo.GetUserByID(ctx, ctx.UserID())
		if err != nil {
			return response.Error(response.ErrNotFound.WithMessage("User not found"))
		}

		// Get session info
		sess, _ := ctx.Session()

		data := ProfilePageData{
			Title: "Profile",
			User: UserData{
				ID:    user.ID,
				Name:  user.Name,
				Email: user.Email,
			},
			Session: SessionInfo{
				ID:        sess.ID,
				IP:        sess.IP,
				UserAgent: sess.UserAgent,
				CreatedAt: sess.CreatedAt.Format("2006-01-02 15:04:05"),
				ExpiresAt: sess.ExpiresAt.Format("2006-01-02 15:04:05"),
			},
		}

		return response.Template(tmpl, data)
	}
}

func logoutHandler() handler.HandlerFunc[*Context] {
	return func(ctx *Context) handler.Response {
		// Delete session
		if err := ctx.Logout(); err != nil {
			// Don't fail logout on error, just redirect anyway
		}

		// Redirect to login page
		return response.RedirectSeeOther("/login")
	}
}
