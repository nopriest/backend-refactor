package models

import "time"

type InvitationStatus string

const (
    InvitationPending  InvitationStatus = "pending"
    InvitationAccepted InvitationStatus = "accepted"
    InvitationDeclined InvitationStatus = "declined"
    InvitationExpired  InvitationStatus = "expired"
)

// OrganizationInvitation is an invite to join an organization
type OrganizationInvitation struct {
    ID             string            `json:"id" db:"id"`
    OrganizationID string            `json:"organization_id" db:"organization_id"`
    Email          string            `json:"email" db:"email"`
    InviterID      string            `json:"inviter_id" db:"inviter_id"`
    Token          string            `json:"token" db:"token"`
    Status         InvitationStatus  `json:"status" db:"status"`
    ExpiresAt      time.Time         `json:"expires_at" db:"expires_at"`
    AcceptedBy     *string           `json:"accepted_by,omitempty" db:"accepted_by"`
    CreatedAt      time.Time         `json:"created_at" db:"created_at"`
    UpdatedAt      time.Time         `json:"updated_at" db:"updated_at"`
}

