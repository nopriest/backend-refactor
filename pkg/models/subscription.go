package models

import (
	"time"
)

// UserTier represents the user subscription tier
type UserTier string

const (
	TierFree  UserTier = "free"
	TierPro   UserTier = "pro"
	TierPower UserTier = "power"
)

// SubscriptionStatus represents the status of a subscription
type SubscriptionStatus string

const (
	StatusActive     SubscriptionStatus = "active"
	StatusCanceled   SubscriptionStatus = "canceled"
	StatusPastDue    SubscriptionStatus = "past_due"
	StatusUnpaid     SubscriptionStatus = "unpaid"
	StatusIncomplete SubscriptionStatus = "incomplete"
)

// UserWithSubscription represents a user with their subscription details
type UserWithSubscription struct {
	User
	Tier               UserTier   `json:"tier" db:"tier"`
	PaddleCustomerID   *string    `json:"paddle_customer_id,omitempty" db:"paddle_customer_id"`
	TrialEndsAt        *time.Time `json:"trial_ends_at,omitempty" db:"trial_ends_at"`
	IsLifetimeMember   bool       `json:"is_lifetime_member" db:"is_lifetime_member"`
	LifetimeMemberType *string    `json:"lifetime_member_type,omitempty" db:"lifetime_member_type"`
}

// SubscriptionPlan represents a subscription plan
type SubscriptionPlan struct {
	ID               string    `json:"id" db:"id"`
	Name             string    `json:"name" db:"name"`
	DisplayName      string    `json:"display_name" db:"display_name"`
	Tier             UserTier  `json:"tier" db:"tier"`
	PriceCents       int       `json:"price_cents" db:"price_cents"`
	Currency         string    `json:"currency" db:"currency"`
	BillingInterval  string    `json:"billing_interval" db:"billing_interval"`
	PaddlePriceID    *string   `json:"paddle_price_id,omitempty" db:"paddle_price_id"`
	AICreditsMonthly int       `json:"ai_credits_monthly" db:"ai_credits_monthly"`
	MaxWorkspaces    *int      `json:"max_workspaces,omitempty" db:"max_workspaces"`
	Features         []string  `json:"features" db:"features"`
	IsActive         bool      `json:"is_active" db:"is_active"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}

// UserSubscription represents a user's subscription
type UserSubscription struct {
	ID                   string             `json:"id" db:"id"`
	UserID               string             `json:"user_id" db:"user_id"`
	PlanID               string             `json:"plan_id" db:"plan_id"`
	PaddleSubscriptionID *string            `json:"paddle_subscription_id,omitempty" db:"paddle_subscription_id"`
	Status               SubscriptionStatus `json:"status" db:"status"`
	CurrentPeriodStart   *time.Time         `json:"current_period_start,omitempty" db:"current_period_start"`
	CurrentPeriodEnd     *time.Time         `json:"current_period_end,omitempty" db:"current_period_end"`
	CancelAtPeriodEnd    bool               `json:"cancel_at_period_end" db:"cancel_at_period_end"`
	CanceledAt           *time.Time         `json:"canceled_at,omitempty" db:"canceled_at"`
	TrialStart           *time.Time         `json:"trial_start,omitempty" db:"trial_start"`
	TrialEnd             *time.Time         `json:"trial_end,omitempty" db:"trial_end"`
	CreatedAt            time.Time          `json:"created_at" db:"created_at"`
	UpdatedAt            time.Time          `json:"updated_at" db:"updated_at"`

	// 关联数据
	Plan *SubscriptionPlan `json:"plan,omitempty"`
}

// AICredits represents a user's AI credits for a billing period
type AICredits struct {
	ID               string    `json:"id" db:"id"`
	UserID           string    `json:"user_id" db:"user_id"`
	CreditsTotal     int       `json:"credits_total" db:"credits_total"`
	CreditsUsed      int       `json:"credits_used" db:"credits_used"`
	CreditsRemaining int       `json:"credits_remaining" db:"credits_remaining"`
	PeriodStart      time.Time `json:"period_start" db:"period_start"`
	PeriodEnd        time.Time `json:"period_end" db:"period_end"`
	CreatedAt        time.Time `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time `json:"updated_at" db:"updated_at"`
}
