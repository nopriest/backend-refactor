package models

import "time"

// Organization represents a collaborative workspace (owner + members)
type Organization struct {
    ID        string    `json:"id" db:"id"`
    Name      string    `json:"name" db:"name"`
    OwnerID   string    `json:"owner_id" db:"owner_id"`
    Description string  `json:"description,omitempty" db:"description"`
    Avatar    string    `json:"avatar,omitempty" db:"avatar"`
    CreatedAt time.Time `json:"created_at" db:"created_at"`
    UpdatedAt time.Time `json:"updated_at" db:"updated_at"`
}

type OrgMemberRole string

const (
    RoleOwner  OrgMemberRole = "owner"
    RoleAdmin  OrgMemberRole = "admin"
    RoleMember OrgMemberRole = "member"
)

// OrganizationMembership relates users to organizations with a role
type OrganizationMembership struct {
    ID             string        `json:"id" db:"id"`
    OrganizationID string        `json:"organization_id" db:"organization_id"`
    UserID         string        `json:"user_id" db:"user_id"`
    Role           OrgMemberRole `json:"role" db:"role"`
    CreatedAt      time.Time     `json:"created_at" db:"created_at"`
}

