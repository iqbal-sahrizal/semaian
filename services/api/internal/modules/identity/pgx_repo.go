package identity

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

type PGXRepository struct {
	pool *pgxpool.Pool
}

func NewPGXRepository(pool *pgxpool.Pool) *PGXRepository {
	return &PGXRepository{pool: pool}
}

func (r *PGXRepository) ResolveOrCreateByExternal(ctx context.Context, issuer, subject string) (User, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return User{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var u User
	err = tx.QueryRow(ctx,
		`SELECT u.id, u.status, u.created_at, u.updated_at
		 FROM identity.users u
		 JOIN identity.external_identities e ON e.user_id = u.id
		 WHERE e.issuer = $1 AND e.subject = $2`,
		issuer, subject,
	).Scan(&u.ID, &u.Status, &u.CreatedAt, &u.UpdatedAt)

	if err == nil {
		if err := tx.Commit(ctx); err != nil {
			return User{}, err
		}
		return u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return User{}, err
	}

	userID := uuid.NewString()
	extID := uuid.NewString()

	err = tx.QueryRow(ctx,
		`INSERT INTO identity.users (id, status) VALUES ($1, 'active')
		 RETURNING id, status, created_at, updated_at`,
		userID,
	).Scan(&u.ID, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return User{}, err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO identity.external_identities (id, user_id, issuer, subject) VALUES ($1, $2, $3, $4)`,
		extID, userID, issuer, subject,
	)
	if err != nil {
		return User{}, err
	}

	_, err = tx.Exec(ctx,
		`INSERT INTO identity.profiles (user_id) VALUES ($1)`,
		userID,
	)
	if err != nil {
		return User{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return User{}, err
	}
	return u, nil
}

func (r *PGXRepository) GetUser(ctx context.Context, userID string) (User, error) {
	var u User
	err := tx.Executor(ctx, r.pool).QueryRow(ctx,
		`SELECT id, status, created_at, updated_at FROM identity.users WHERE id = $1`,
		userID,
	).Scan(&u.ID, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *PGXRepository) UpdateStatus(ctx context.Context, userID string, status Status) (User, error) {
	var u User
	err := tx.Executor(ctx, r.pool).QueryRow(ctx,
		`UPDATE identity.users SET status = $2, updated_at = now() WHERE id = $1
		 RETURNING id, status, created_at, updated_at`,
		userID, string(status),
	).Scan(&u.ID, &u.Status, &u.CreatedAt, &u.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrNotFound
	}
	return u, err
}

func (r *PGXRepository) GetProfile(ctx context.Context, userID string) (Profile, error) {
	var p Profile
	err := r.pool.QueryRow(ctx,
		`SELECT user_id, display_name, avatar_ref, preferred_locale, timezone, updated_at
		 FROM identity.profiles WHERE user_id = $1`,
		userID,
	).Scan(&p.UserID, &p.DisplayName, &p.AvatarRef, &p.PreferredLocale, &p.Timezone, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	return p, err
}

func (r *PGXRepository) UpdateProfile(ctx context.Context, userID string, upd ProfileUpdate) (Profile, error) {
	var p Profile
	err := r.pool.QueryRow(ctx,
		`UPDATE identity.profiles
		 SET display_name = COALESCE($2, display_name),
		     avatar_ref = COALESCE($3, avatar_ref),
		     preferred_locale = COALESCE($4, preferred_locale),
		     timezone = COALESCE($5, timezone),
		     updated_at = now()
		 WHERE user_id = $1
		 RETURNING user_id, display_name, avatar_ref, preferred_locale, timezone, updated_at`,
		userID, upd.DisplayName, upd.AvatarRef, upd.PreferredLocale, upd.Timezone,
	).Scan(&p.UserID, &p.DisplayName, &p.AvatarRef, &p.PreferredLocale, &p.Timezone, &p.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	return p, err
}
