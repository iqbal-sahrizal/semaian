package organization

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

type OrgStatus string

const (
	OrgStatusActive      OrgStatus = "active"
	OrgStatusSuspended   OrgStatus = "suspended"
	OrgStatusDeactivated OrgStatus = "deactivated"
)

type Role string

const (
	RoleOwner  Role = "owner"
	RoleAdmin  Role = "admin"
	RoleMember Role = "member"
)

type MembershipStatus string

const (
	MembershipActive   MembershipStatus = "active"
	MembershipInactive MembershipStatus = "inactive"
)

type Permission string

const (
	PermRead          Permission = "organization.read"
	PermUpdate        Permission = "organization.update"
	PermManageMembers Permission = "organization.manage_members"
	PermManageRoles   Permission = "organization.manage_roles"
	PermDelete        Permission = "organization.delete"
)

var rolePermissions = map[Role]map[Permission]bool{
	RoleOwner: {
		PermRead:          true,
		PermUpdate:        true,
		PermManageMembers: true,
		PermManageRoles:   true,
		PermDelete:        true,
	},
	RoleAdmin: {
		PermRead:          true,
		PermUpdate:        true,
		PermManageMembers: true,
		PermManageRoles:   true,
	},
	RoleMember: {
		PermRead: true,
	},
}

// HasPermission checks if a role grants a given permission.
func HasPermission(role Role, perm Permission) bool {
	return rolePermissions[role][perm]
}

var (
	ErrNotFound         = errors.New("organization or membership not found")
	ErrForbidden        = errors.New("forbidden")
	ErrOrgInactive      = errors.New("organization is suspended or deactivated")
	ErrLastOwner        = errors.New("cannot remove or demote the last active owner")
	ErrConflict         = errors.New("organization slug or membership already exists")
	ErrInvalidInput     = errors.New("invalid input")
	validSlugExpression = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}[a-z0-9]$`)
)

type Organization struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Slug      string    `json:"slug"`
	Status    OrgStatus `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Membership struct {
	ID             string           `json:"id"`
	OrganizationID string           `json:"organization_id"`
	UserID         string           `json:"user_id"`
	Role           Role             `json:"role"`
	Status         MembershipStatus `json:"status"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
}

type Repository interface {
	ListByUser(ctx context.Context, userID string) ([]Organization, error)
	CreateWithOwner(ctx context.Context, name, slug, ownerUserID string) (Organization, Membership, error)
	GetByID(ctx context.Context, orgID string) (Organization, error)
	Update(ctx context.Context, orgID, name string) (Organization, error)
	GetMembership(ctx context.Context, orgID, userID string) (Membership, error)
	ListMembers(ctx context.Context, orgID string) ([]Membership, error)
	ChangeMemberRole(ctx context.Context, orgID, targetUserID string, newRole Role) (Membership, error)
	RemoveMember(ctx context.Context, orgID, targetUserID string) error
	TransferOwnership(ctx context.Context, orgID, currentOwnerID, newOwnerID string) error
}

type Service struct {
	repo     Repository
	audit    audit.Recorder
	txRunner tx.Runner
}

func NewService(repo Repository, auditRec audit.Recorder) *Service {
	return &Service{repo: repo, audit: auditRec, txRunner: tx.NoopRunner{}}
}

func (s *Service) WithTxRunner(r tx.Runner) *Service {
	if r != nil {
		s.txRunner = r
	}
	return s
}

func (s *Service) authorize(ctx context.Context, orgID, userID string, perm Permission) (Organization, Membership, error) {
	org, err := s.repo.GetByID(ctx, orgID)
	if err != nil {
		return Organization{}, Membership{}, err
	}
	mem, err := s.repo.GetMembership(ctx, orgID, userID)
	if err != nil || mem.Status != MembershipActive {
		return Organization{}, Membership{}, ErrForbidden
	}
	if org.Status != OrgStatusActive {
		return Organization{}, Membership{}, ErrOrgInactive
	}
	if !HasPermission(mem.Role, perm) {
		return Organization{}, Membership{}, ErrForbidden
	}
	return org, mem, nil
}

func (s *Service) ListAccessible(ctx context.Context, userID string) ([]Organization, error) {
	return s.repo.ListByUser(ctx, userID)
}

func (s *Service) Create(ctx context.Context, actorUserID, name, slug, correlationID string) (Organization, error) {
	name = strings.TrimSpace(name)
	slug = strings.TrimSpace(strings.ToLower(slug))
	if name == "" || len(name) > 120 || !validSlugExpression.MatchString(slug) {
		return Organization{}, ErrInvalidInput
	}
	org, _, err := s.repo.CreateWithOwner(ctx, name, slug, actorUserID)
	if err != nil {
		return Organization{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID:        &actorUserID,
		OrganizationID: &org.ID,
		Action:         "organization.created",
		TargetType:     "organization",
		TargetID:       org.ID,
		CorrelationID:  correlationID,
	})
	return org, nil
}

func (s *Service) Get(ctx context.Context, actorUserID, orgID string) (Organization, error) {
	org, _, err := s.authorize(ctx, orgID, actorUserID, PermRead)
	return org, err
}

func (s *Service) Update(ctx context.Context, actorUserID, orgID, name, correlationID string) (Organization, error) {
	if _, _, err := s.authorize(ctx, orgID, actorUserID, PermUpdate); err != nil {
		return Organization{}, err
	}
	name = strings.TrimSpace(name)
	if name == "" || len(name) > 120 {
		return Organization{}, ErrInvalidInput
	}
	updated, err := s.repo.Update(ctx, orgID, name)
	if err != nil {
		return Organization{}, err
	}
	_ = s.audit.Record(ctx, audit.Event{
		ActorID:        &actorUserID,
		OrganizationID: &orgID,
		Action:         "organization.updated",
		TargetType:     "organization",
		TargetID:       orgID,
		CorrelationID:  correlationID,
	})
	return updated, nil
}

func (s *Service) ListMembers(ctx context.Context, actorUserID, orgID string) ([]Membership, error) {
	if _, _, err := s.authorize(ctx, orgID, actorUserID, PermRead); err != nil {
		return nil, err
	}
	return s.repo.ListMembers(ctx, orgID)
}

func (s *Service) ChangeMemberRole(ctx context.Context, actorUserID, orgID, targetUserID string, newRole Role, correlationID string) (Membership, error) {
	var updated Membership
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		_, actorMem, err := s.authorize(txCtx, orgID, actorUserID, PermManageRoles)
		if err != nil {
			return err
		}
		if newRole != RoleAdmin && newRole != RoleMember {
			return ErrInvalidInput
		}
		if actorMem.Role != RoleOwner {
			targetMem, err := s.repo.GetMembership(txCtx, orgID, targetUserID)
			if err != nil {
				return err
			}
			if targetMem.Role == RoleOwner {
				return ErrForbidden
			}
		}
		updated, err = s.repo.ChangeMemberRole(txCtx, orgID, targetUserID, newRole)
		if err != nil {
			return err
		}
		return s.audit.Record(txCtx, audit.Event{
			ActorID:        &actorUserID,
			OrganizationID: &orgID,
			Action:         "organization.member.role_changed",
			TargetType:     "membership",
			TargetID:       updated.ID,
			CorrelationID:  correlationID,
			Metadata:       map[string]any{"new_role": string(newRole)},
		})
	})
	if err != nil {
		return Membership{}, err
	}
	return updated, nil
}

func (s *Service) RemoveMember(ctx context.Context, actorUserID, orgID, targetUserID, correlationID string) error {
	return s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		_, actorMem, err := s.authorize(txCtx, orgID, actorUserID, PermManageMembers)
		if err != nil {
			return err
		}
		if actorMem.Role != RoleOwner {
			targetMem, err := s.repo.GetMembership(txCtx, orgID, targetUserID)
			if err != nil {
				return err
			}
			if targetMem.Role == RoleOwner {
				return ErrForbidden
			}
		}
		if err := s.repo.RemoveMember(txCtx, orgID, targetUserID); err != nil {
			return err
		}
		return s.audit.Record(txCtx, audit.Event{
			ActorID:        &actorUserID,
			OrganizationID: &orgID,
			Action:         "organization.member.removed",
			TargetType:     "user",
			TargetID:       targetUserID,
			CorrelationID:  correlationID,
		})
	})
}

func (s *Service) TransferOwnership(ctx context.Context, actorUserID, orgID, newOwnerUserID, correlationID string) error {
	return s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		_, actorMem, err := s.authorize(txCtx, orgID, actorUserID, PermDelete)
		if err != nil {
			return err
		}
		if actorMem.Role != RoleOwner || actorUserID == newOwnerUserID {
			return ErrForbidden
		}
		if err := s.repo.TransferOwnership(txCtx, orgID, actorUserID, newOwnerUserID); err != nil {
			return err
		}
		return s.audit.Record(txCtx, audit.Event{
			ActorID:        &actorUserID,
			OrganizationID: &orgID,
			Action:         "organization.ownership.transferred",
			TargetType:     "user",
			TargetID:       newOwnerUserID,
			CorrelationID:  correlationID,
		})
	})
}
