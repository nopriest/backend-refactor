package models

import (
	"time"
)

// SavedTab represents a saved browser tab (compatible with existing database)
type SavedTab struct {
	ID            string                 `json:"id"`
	Title         string                 `json:"title"`
	URL           string                 `json:"url"`
	FavIconURL    *string                `json:"favIconUrl,omitempty"`
	OriginalTitle string                 `json:"originalTitle"`
	Domain        string                 `json:"domain"`
	CreatedAt     time.Time              `json:"createdAt"`
	LastAccessed  *time.Time             `json:"lastAccessed,omitempty"`
	Tags          []string               `json:"tags"`
	Metadata      map[string]interface{} `json:"metadata,omitempty"`
}

// Tab represents a browser tab (for new API compatibility)
type Tab struct {
	ID         int    `json:"id"`
	Title      string `json:"title"`
	URL        string `json:"url"`
	FaviconURL string `json:"faviconUrl,omitempty"`
	Active     bool   `json:"active,omitempty"`
	Pinned     bool   `json:"pinned,omitempty"`
	Index      int    `json:"index,omitempty"`
}

// TabGroup represents a group of tabs (compatible with existing database)
type TabGroup struct {
	ID                  string     `json:"id"`
	UserID              string     `json:"userId"`
	Name                string     `json:"name"`
	Description         *string    `json:"description,omitempty"`
	CreatedAt           time.Time  `json:"createdAt"`
	UpdatedAt           time.Time  `json:"updatedAt"`
	Tabs                []SavedTab `json:"tabs"`
	Color               *string    `json:"color,omitempty"`
	AutoClassify        bool       `json:"autoClassify"`
	ClassificationRules []string   `json:"classificationRules"`
}

// Snapshot represents a saved tab snapshot
type Snapshot struct {
	ID         string     `json:"id" db:"id"`
	UserID     string     `json:"user_id" db:"user_id"`
	Name       string     `json:"name" db:"name"`
	TabGroups  []TabGroup `json:"tab_groups" db:"tab_groups"`
	GroupCount int        `json:"group_count" db:"group_count"`
	TabCount   int        `json:"tab_count" db:"tab_count"`
	CreatedAt  time.Time  `json:"created_at" db:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at" db:"updated_at"`
}

// SnapshotInfo represents snapshot metadata
type SnapshotInfo struct {
	Name       string `json:"name"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
	TabCount   int    `json:"tab_count"`
	GroupCount int    `json:"group_count"`
}

// LoadSnapshotResponse represents the response for loading a snapshot
type LoadSnapshotResponse struct {
	Name      string     `json:"name"`
	TabGroups []TabGroup `json:"tabGroups"`
	CreatedAt string     `json:"createdAt"`
	UpdatedAt string     `json:"updatedAt"`
}
