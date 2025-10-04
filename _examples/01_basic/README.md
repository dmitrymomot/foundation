# Basic Example - Session-Based Authentication with JWT

This example demonstrates a complete authentication flow using the Foundation framework with session management and JWT transport.

## Features

- User registration (signup)
- User authentication (login)
- JWT-based session transport with access and refresh tokens
- Protected endpoints requiring authentication
- Session middleware for automatic session loading
- Password management
- PostgreSQL-backed session storage

## Prerequisites

- Go 1.21+
- Docker and Docker Compose (for PostgreSQL)

## Running the Application

1. Start everything with one command:

```bash
make up
```

Or manually:

```bash
# Start PostgreSQL
docker-compose up -d

# Run the application
go run .
```

The server starts on `http://localhost:3000` by default.

To stop and clean up everything:

```bash
make down
```

## API Endpoints

### Health Checks

#### Liveness Check

```bash
# Set your port (default: 3000)
HTTP_PORT=3000

curl http://localhost:8081/live
```

#### Readiness Check

```bash
curl http://localhost:8081/ready
```

---

### Authentication Endpoints

#### 1. Signup

Create a new user account.

**Request:**

```bash
curl -X POST http://localhost:8081/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "name": "John Doe",
    "email": "john@example.com",
    "password": "SecurePass123!@#"
  }'
```

**Response:**

```json
{
    "user": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "John Doe",
        "email": "john@example.com"
    },
    "tokens": {
        "access_token": "eyJhbGci...",
        "refresh_token": "eyJhbGci...",
        "token_type": "Bearer",
        "expires_in": 3600,
        "expires_at": "2025-01-04T12:00:00Z"
    }
}
```

**Password Requirements:**

- Minimum 8 characters
- At least one uppercase letter
- At least one lowercase letter
- At least one digit
- At least one special character
- Not a commonly used password

---

#### 2. Login

Authenticate with existing credentials.

**Request:**

```bash
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "john@example.com",
    "password": "SecurePass123!@#"
  }'
```

**Response:**

```json
{
    "user": {
        "id": "550e8400-e29b-41d4-a716-446655440000",
        "name": "John Doe",
        "email": "john@example.com"
    },
    "tokens": {
        "access_token": "eyJhbGci...",
        "refresh_token": "eyJhbGci...",
        "token_type": "Bearer",
        "expires_in": 3600,
        "expires_at": "2025-01-04T12:00:00Z"
    }
}
```

---

#### 3. Refresh Tokens

Get a new access token using a refresh token.

**Request:**

```bash
curl -X POST http://localhost:8081/auth/refresh \
  -H "Content-Type: application/json" \
  -d '{
    "refresh_token": "eyJhbGci..."
  }'
```

**Response:**

```json
{
    "access_token": "eyJhbGci...",
    "refresh_token": "eyJhbGci...",
    "token_type": "Bearer",
    "expires_in": 3600,
    "expires_at": "2025-01-04T13:00:00Z"
}
```

---

### Protected Endpoints

All protected endpoints require a valid access token in the `Authorization` header.

#### 4. Get Profile

Retrieve the authenticated user's profile.

**Request:**

```bash
curl http://localhost:8081/api/profile \
  -H "Authorization: Bearer eyJhbGci..."
```

**Response:**

```json
{
    "id": "550e8400-e29b-41d4-a716-446655440000",
    "name": "John Doe",
    "email": "john@example.com"
}
```

---

#### 5. Update Password

Change the user's password.

**Request:**

```bash
curl -X PUT http://localhost:8081/api/profile/password \
  -H "Authorization: Bearer eyJhbGci..." \
  -H "Content-Type: application/json" \
  -d '{
    "old_password": "SecurePass123!@#",
    "new_password": "NewSecurePass456!@#"
  }'
```

**Response:**

```json
{
    "message": "Password updated successfully"
}
```

---

#### 6. Logout

End the current session.

**Request:**

```bash
curl -X POST http://localhost:8081/api/auth/logout \
  -H "Authorization: Bearer eyJhbGci..."
```

**Response:**

```json
{
    "message": "Logged out successfully"
}
```

---

## Complete Workflow Example

Here's a complete workflow demonstrating the authentication flow:

```bash
# 1. Signup
SIGNUP_RESPONSE=$(curl -s -X POST http://localhost:8081/auth/signup \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Jane Smith",
    "email": "jane@example.com",
    "password": "MySecurePass123!@#"
  }')

# Extract access and refresh tokens
ACCESS_TOKEN=$(echo $SIGNUP_RESPONSE | jq -r '.tokens.access_token')
REFRESH_TOKEN=$(echo $SIGNUP_RESPONSE | jq -r '.tokens.refresh_token')

echo "Access Token: $ACCESS_TOKEN"
echo "Refresh Token: $REFRESH_TOKEN"

# 2. Get profile using access token
curl http://localhost:8081/api/profile \
  -H "Authorization: Bearer $ACCESS_TOKEN"

# 3. Update password
curl -X PUT http://localhost:8081/api/profile/password \
  -H "Authorization: Bearer $ACCESS_TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "old_password": "MySecurePass123!@#",
    "new_password": "UpdatedPass456!@#"
  }'

# 4. Refresh tokens
REFRESH_RESPONSE=$(curl -s -X POST http://localhost:8081/auth/refresh \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\": \"$REFRESH_TOKEN\"}")

NEW_ACCESS_TOKEN=$(echo $REFRESH_RESPONSE | jq -r '.access_token')
echo "New Access Token: $NEW_ACCESS_TOKEN"

# 5. Use new access token
curl http://localhost:8081/api/profile \
  -H "Authorization: Bearer $NEW_ACCESS_TOKEN"

# 6. Logout
curl -X POST http://localhost:8081/api/auth/logout \
  -H "Authorization: Bearer $NEW_ACCESS_TOKEN"

# 7. Login with updated password
curl -X POST http://localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "email": "jane@example.com",
    "password": "UpdatedPass456!@#"
  }'
```

---

## Error Responses

The API returns standard HTTP status codes with error details:

### 400 Bad Request

```json
{
    "error": "Bad Request",
    "message": "Failed to parse request body",
    "details": {
        "errors": {
            "password": "password must be at least 8 characters long"
        }
    }
}
```

### 401 Unauthorized

```json
{
    "error": "Unauthorized",
    "message": "Invalid credentials"
}
```

### 404 Not Found

```json
{
    "error": "Not Found",
    "message": "User not found"
}
```

### 409 Conflict

```json
{
    "error": "Conflict",
    "message": "Email already exists"
}
```

### 500 Internal Server Error

```json
{
    "error": "Internal Server Error"
}
```

---

## Configuration

The application uses environment variables for configuration. Create a `.env` file:

```env
APP_NAME=basic-example
HTTP_HOST=0.0.0.0
HTTP_PORT=3000

DB_HOST=localhost
DB_PORT=5432
DB_USER=postgres
DB_PASSWORD=postgres
DB_NAME=foundation_example
DB_SSLMODE=disable

JWT_SIGNING_KEY=your-secret-key-min-32-chars-long!
ACCESS_TOKEN_TTL=1h
SESSION_TTL=720h
SESSION_TOUCH_INTERVAL=5m
```

---

## Architecture Highlights

This example demonstrates:

1. **Session Management**: Database-backed sessions with automatic touch/expiration
2. **JWT Transport**: Stateless tokens with session linkage via JTI claim
3. **Context Convenience Methods**: Clean API with `ctx.Auth(userID)`, `ctx.Logout()`
4. **Session Middleware**: Automatic session loading from JWT bearer tokens
5. **Type-Safe Handlers**: Using Foundation's generic handler pattern
6. **Custom Context**: Extended context with session and transport helpers

## Database Schema

The example uses PostgreSQL with the following tables:

- `users`: User accounts with hashed passwords
- `sessions`: Session storage with token rotation and expiration

Migrations are automatically applied on application startup.
