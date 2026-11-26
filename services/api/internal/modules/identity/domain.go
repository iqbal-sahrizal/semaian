package identity

import (
	"context"
	"errors"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/semaian/platform/services/api/internal/modules/audit"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

type Status string

const (
	StatusActive      Status = "active"
	StatusSuspended   Status = "suspended"
	StatusDeactivated Status = "deactivated"
)

var (
	ErrNotFound          = errors.New("identity not found")
	ErrInactiveIdentity  = errors.New("identity is suspended or deactivated")
	ErrInvalidTransition = errors.New("invalid identity status transition")
	ErrInvalidInput      = errors.New("invalid input")
)

type User struct {
	ID        string    `json:"id"`
	Status    Status    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Profile struct {
	UserID          string    `json:"user_id"`
	DisplayName     string    `json:"display_name"`
	AvatarRef       string    `json:"avatar_ref,omitempty"`
	PreferredLocale string    `json:"preferred_locale"`
	Timezone        string    `json:"timezone"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type ProfileUpdate struct {
	DisplayName     *string `json:"display_name"`
	AvatarRef       *string `json:"avatar_ref"`
	PreferredLocale *string `json:"preferred_locale"`
	Timezone        *string `json:"timezone"`
}

type Repository interface {
	ResolveOrCreateByExternal(ctx context.Context, issuer, subject string) (User, error)
	GetUser(ctx context.Context, userID string) (User, error)
	UpdateStatus(ctx context.Context, userID string, status Status) (User, error)
	GetProfile(ctx context.Context, userID string) (Profile, error)
	UpdateProfile(ctx context.Context, userID string, upd ProfileUpdate) (Profile, error)
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

// ResolveExternalIdentity resolves or creates an internal user from verified OIDC (iss, sub).
// Suspended or deactivated users are rejected.
func (s *Service) ResolveExternalIdentity(ctx context.Context, issuer, subject string) (User, error) {
	if strings.TrimSpace(issuer) == "" || strings.TrimSpace(subject) == "" {
		return User{}, ErrInvalidInput
	}
	u, err := s.repo.ResolveOrCreateByExternal(ctx, issuer, subject)
	if err != nil {
		return User{}, err
	}
	if u.Status != StatusActive {
		return User{}, ErrInactiveIdentity
	}
	return u, nil
}

func (s *Service) GetMe(ctx context.Context, userID string) (User, error) {
	u, err := s.repo.GetUser(ctx, userID)
	if err != nil {
		return User{}, err
	}
	if u.Status != StatusActive {
		return User{}, ErrInactiveIdentity
	}
	return u, nil
}

func (s *Service) GetProfile(ctx context.Context, userID string) (Profile, error) {
	if _, err := s.GetMe(ctx, userID); err != nil {
		return Profile{}, err
	}
	return s.repo.GetProfile(ctx, userID)
}

var (
	validLocaleRe     = regexp.MustCompile(`^[a-zA-Z]{2,3}(-[a-zA-Z0-9]{2,8})*$`)
	validStorageKeyRe = regexp.MustCompile(`^[a-zA-Z0-9/_.-]{1,512}$`)
)

func (s *Service) UpdateProfile(ctx context.Context, userID string, upd ProfileUpdate) (Profile, error) {
	if _, err := s.GetMe(ctx, userID); err != nil {
		return Profile{}, err
	}
	if upd.DisplayName != nil {
		trimmed := strings.TrimSpace(*upd.DisplayName)
		if len(trimmed) > 100 {
			return Profile{}, ErrInvalidInput
		}
		upd.DisplayName = &trimmed
	}
	if upd.AvatarRef != nil {
		ref := strings.TrimSpace(*upd.AvatarRef)
		if len(ref) > 512 {
			return Profile{}, ErrInvalidInput
		}
		if ref != "" {
			if strings.Contains(ref, "..") || strings.ContainsAny(ref, "<>\"'") {
				return Profile{}, ErrInvalidInput
			}
			if strings.Contains(ref, "://") {
				u, err := url.Parse(ref)
				if err != nil || u.Scheme != "https" || u.Host == "" {
					return Profile{}, ErrInvalidInput
				}
			} else if !validStorageKeyRe.MatchString(ref) {
				return Profile{}, ErrInvalidInput
			}
		}
		upd.AvatarRef = &ref
	}
	if upd.PreferredLocale != nil {
		loc := strings.TrimSpace(*upd.PreferredLocale)
		if len(loc) > 16 || !validLocaleRe.MatchString(loc) {
			return Profile{}, ErrInvalidInput
		}
		upd.PreferredLocale = &loc
	}
	if upd.Timezone != nil {
		tz := strings.TrimSpace(*upd.Timezone)
		if tz == "" || len(tz) > 64 {
			return Profile{}, ErrInvalidInput
		}
		if _, err := time.LoadLocation(tz); err != nil {
			return Profile{}, ErrInvalidInput
		}
		upd.Timezone = &tz
	}
	return s.repo.UpdateProfile(ctx, userID, upd)
}

func (s *Service) SetStatus(ctx context.Context, actorID, targetUserID string, next Status, correlationID string) (User, error) {
	var updated User
	err := s.txRunner.WithinTx(ctx, func(txCtx context.Context) error {
		u, err := s.repo.GetUser(txCtx, targetUserID)
		if err != nil {
			return err
		}
		if u.Status == StatusDeactivated && next != StatusDeactivated {
			return ErrInvalidTransition
		}
		updated, err = s.repo.UpdateStatus(txCtx, targetUserID, next)
		if err != nil {
			return err
		}
		return s.audit.Record(txCtx, audit.Event{
			ActorID:       &actorID,
			Action:        "identity.status." + string(next),
			TargetType:    "user",
			TargetID:      targetUserID,
			CorrelationID: correlationID,
		})
	})
	if err != nil {
		return User{}, err
	}
	return updated, nil
}
