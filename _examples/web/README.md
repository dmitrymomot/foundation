# Foundation Web Example

A simple web application demonstrating cookie-based authentication with HTML templates using the Foundation framework.

## Features

- **Cookie-based sessions** - Sessions stored server-side, cookie contains signed token
- **HTML templates** - Server-rendered HTML using Go's `html/template`
- **Simple auth flow** - Signup → Login → Profile with logout
- **Custom error pages** - HTML error pages for better UX
- **Auto-redirect** - Unauthenticated users redirected to login page

## Routes

- `GET /signup` - Signup page (form)
- `POST /signup` - Signup form submission
- `GET /login` - Login page (form)
- `POST /login` - Login form submission
- `GET /` - Profile page (protected, shows user and session info)
- `POST /logout` - Logout (protected)
- `GET /live` - Liveness check
- `GET /ready` - Readiness check (includes DB health)

## Prerequisites

- Go 1.23+
- Docker & Docker Compose (for PostgreSQL)
- [sqlc](https://sqlc.dev/) for generating database code

## Setup

### 1. Generate database code

```bash
# From the project root
make sqlc
```

Or manually:

```bash
cd _examples/web
sqlc generate
```

### 2. Start PostgreSQL

```bash
cd _examples/web
docker-compose up -d
```

### 3. Set environment variables

Create a `.env` file in `_examples/web/`:

```bash
# Cookie secrets (comma-separated for key rotation)
COOKIE_SECRETS="your-secret-key-min-32-chars-long,optional-old-key-for-rotation"

# Database
DATABASE_DSN="postgres://foundation:foundation@localhost:5433/foundation?sslmode=disable"

# Server (optional)
SERVER_PORT=8082
SERVER_READ_TIMEOUT=10s
SERVER_WRITE_TIMEOUT=10s
```

**Important:** The `COOKIE_SECRETS` should be at least 32 characters long. For production, use a cryptographically random string.

### 4. Run the application

From the example directory:

```bash
go run .
```

Or from the project root:

```bash
go run ./_examples/web
```

The server will start on `http://localhost:8082`

## Usage

1. **Visit** `http://localhost:8082/`
    - You'll be redirected to `/login` (not authenticated)

2. **Create account** - Click "Sign up" or go to `/signup`
    - Fill in name, email, password (min 8 chars, must be strong)
    - Submit → automatically logged in → redirected to profile

3. **View profile** - The home page (`/`) shows:
    - User information (ID, name, email)
    - Session information (ID, IP, user agent, timestamps)
    - Logout button

4. **Logout** - Click "Log Out"
    - Session deleted → redirected to `/login`

5. **Login again** - Go to `/login`
    - Enter email and password
    - Submit → redirected to profile

## Architecture

### Cookie-based Authentication

Unlike the API example which uses JWT tokens, this example uses cookie-based sessions:

- **Session storage:** PostgreSQL (same sessions table)
- **Cookie:** Contains signed session token (not the full session data)
- **Security:** Cookies are HTTP-only, signed, and can be encrypted
- **No token refresh:** Sessions auto-extend on activity via touch interval

### Template Structure

Templates use Go's standard `html/template` with layout inheritance:

- `layout.html` - Base layout with styles
- `signup.html`, `login.html`, `profile.html` - Page templates
- `error.html` - Error page template

Each template defines a `{{define "content"}}` block that gets injected into the layout.

### Error Handling

Custom error handler (`WithErrorHandler`) converts errors to HTML:

- Detects `response.Error` types
- Renders error template with status code and message
- Falls back to plain text if template rendering fails

### Session Middleware

The session middleware for protected routes includes `OnUnauthorized` callback:

```go
OnUnauthorized: func(ctx *Context) {
    response.Redirect("/login")(ctx.ResponseWriter(), ctx.Request())
}
```

This provides a better UX for HTML apps compared to returning 401 JSON responses.

## Development

### Database migrations

Migrations run automatically on application start. Migration files are in `db/migrations/`.

To create a new migration:

```bash
# Create migration files with timestamp
./scripts/create-migration.sh create_new_table
```

### Regenerate database code

After modifying SQL queries in `db/queries/`:

```bash
make sqlc
# or
sqlc generate
```

### Clean up

Stop PostgreSQL:

```bash
docker-compose down
```

Remove database volume:

```bash
docker-compose down -v
```

## Comparison with API Example

| Feature             | API Example                       | Web Example                  |
| ------------------- | --------------------------------- | ---------------------------- |
| **Transport**       | JWT (Authorization header)        | Cookie (HTTP-only, signed)   |
| **Response format** | JSON                              | HTML (templates)             |
| **Auth method**     | Bearer token                      | Session cookie               |
| **Refresh flow**    | Explicit `/auth/refresh` endpoint | Automatic via touch interval |
| **Error format**    | JSON error response               | HTML error page              |
| **Unauthenticated** | 401 Unauthorized                  | Redirect to `/login`         |
| **Client type**     | API clients, mobile apps          | Web browsers                 |

Both examples share:

- Same database schema and migrations
- Same session storage implementation
- Same core validation and sanitization
- Same health check endpoints

## Security Notes

1. **Cookie secrets:** Use strong, random secrets in production (32+ bytes)
2. **HTTPS:** Always use HTTPS in production for secure cookies
3. **CSRF:** Consider adding CSRF protection for state-changing operations
4. **Password strength:** Enforced via validators (`strong_password`, `not_common_password`)
5. **Session expiry:** Sessions expire after 7 days (configurable via `SESSION_TTL`)

## Next Steps

- Add CSRF protection middleware
- Add remember-me functionality (longer session TTL)
- Add email verification flow
- Add password reset flow
- Add profile editing
- Add HTMX for dynamic interactions without full page reloads
