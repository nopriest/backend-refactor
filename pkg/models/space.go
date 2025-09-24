package models

import "time"

// Space is a container under an Organization
type Space struct {
    ID             string    `json:"id" db:"id"`
    OrganizationID string    `json:"organization_id" db:"organization_id"`
    Name           string    `json:"name" db:"name"`
    Description    string    `json:"description,omitempty" db:"description"`
    IsDefault      bool      `json:"is_default" db:"is_default"`
    CreatedAt      time.Time `json:"created_at" db:"created_at"`
    UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// SpacePermission controls per-member editing capability in a space
type SpacePermission struct {
    ID        string    `json:"id" db:"id"`
    SpaceID   string    `json:"space_id" db:"space_id"`
    UserID    string    `json:"user_id" db:"user_id"`
    CanEdit   bool      `json:"can_edit" db:"can_edit"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

