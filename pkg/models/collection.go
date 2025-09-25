package models

import "time"

// Collection represents a group of saved tabs/items under a Space
type Collection struct {
    ID          string    `json:"id" db:"id"`
    SpaceID     string    `json:"space_id" db:"space_id"`
    Name        string    `json:"name" db:"name"`
    Description string    `json:"description,omitempty" db:"description"`
    Color       string    `json:"color,omitempty" db:"color"`
    Icon        string    `json:"icon,omitempty" db:"icon"`
    Position    int       `json:"position" db:"position"`
    CreatedAt   time.Time `json:"created_at" db:"created_at"`
    UpdatedAt   time.Time `json:"updated_at" db:"updated_at"`
    DeletedAt   *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}
