package models

import "time"

// CollectionItem represents a saved tab/item under a collection
type CollectionItem struct {
    ID              string     `json:"id" db:"id"`
    CollectionID    string     `json:"collection_id" db:"collection_id"`
    Title           string     `json:"title" db:"title"`
    URL             string     `json:"url,omitempty" db:"url"`
    FavIconURL      string     `json:"fav_icon_url,omitempty" db:"fav_icon_url"`
    OriginalTitle   string     `json:"original_title,omitempty" db:"original_title"`
    AIGeneratedTitle string    `json:"ai_generated_title,omitempty" db:"ai_generated_title"`
    Domain          string     `json:"domain,omitempty" db:"domain"`
    Metadata        []byte     `json:"metadata,omitempty" db:"metadata"`
    Position        int        `json:"position" db:"position"`
    CreatedAt       time.Time  `json:"created_at" db:"created_at"`
    UpdatedAt       time.Time  `json:"updated_at" db:"updated_at"`
    DeletedAt       *time.Time `json:"deleted_at,omitempty" db:"deleted_at"`
}

