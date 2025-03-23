package models

import (
	"time"

	"github.com/google/uuid"
)

// User represents a user of the system
type User struct {
	ID        uuid.UUID `json:"id" db:"id"`
	Email     string    `json:"email" db:"email"`
	Name      string    `json:"name" db:"name"`
	Password  string    `json:"-" db:"password"` // Never expose password in JSON
	Role      string    `json:"role" db:"role"`
	CreatedAt time.Time `json:"created_at" db:"created_at"`
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

// UserCredentials represents login credentials
type UserCredentials struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
}

// AuthResponse represents the authentication response with JWT token
type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}
