package tenant

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OrgPlan string

const (
	OrgPlanFree       OrgPlan = "free"
	OrgPlanStarter    OrgPlan = "starter"
	OrgPlanPro        OrgPlan = "pro"
	OrgPlanEnterprise OrgPlan = "enterprise"
)

type Org struct {
	ID          uuid.UUID
	Name        string
	Slug        string
	Plan        OrgPlan
	OwnerUserID uuid.UUID
	LogoURL     string
	Website     string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

func NewOrg(name, slug string, ownerUserID uuid.UUID) (*Org, error) {
	if name == "" {
		return nil, fmt.Errorf("NewOrg: %w", ErrInvalidName)
	}
	if !isValidSlug(slug) {
		return nil, fmt.Errorf("NewOrg: %w", ErrInvalidSlug)
	}
	now := time.Now().UTC()
	return &Org{
		ID:          uuid.New(),
		Name:        name,
		Slug:        slug,
		Plan:        OrgPlanFree,
		OwnerUserID: ownerUserID,
		CreatedAt:   now,
		UpdatedAt:   now,
	}, nil
}
