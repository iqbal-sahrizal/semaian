package organization

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/semaian/platform/services/api/internal/platform/tx"
)

type PGXRepository struct {
	pool *pgxpool.Pool
}

func NewPGXRepository(pool *pgxpool.Pool) *PGXRepository {
	return &PGXRepository{pool: pool}
}

func (r *PGXRepository) ListByUser(ctx context.Context, userID string) ([]Organization, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT o.id, o.name, o.slug, o.status, o.created_at, o.updated_at
		 FROM organization.organizations o
		 JOIN organization.memberships m ON m.organization_id = o.id
		 WHERE m.user_id = $1 AND m.status = 'active' AND o.status = 'active'
		 ORDER BY o.created_at DESC`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Slug, &o.Status, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

func (r *PGXRepository) CreateWithOwner(ctx context.Context, name, slug, ownerUserID string) (Organization, Membership, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return Organization{}, Membership{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	orgID := uuid.NewString()
	memID := uuid.NewString()

	var org Organization
	err = tx.QueryRow(ctx,
		`INSERT INTO organization.organizations (id, name, slug, status)
		 VALUES ($1, $2, $3, 'active')
		 RETURNING id, name, slug, status, created_at, updated_at`,
		orgID, name, slug,
	).Scan(&org.ID, &org.Name, &org.Slug, &org.Status, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return Organization{}, Membership{}, ErrConflict
		}
		return Organization{}, Membership{}, err
	}

	var mem Membership
	err = tx.QueryRow(ctx,
		`INSERT INTO organization.memberships (id, organization_id, user_id, role, status)
		 VALUES ($1, $2, $3, 'owner', 'active')
		 RETURNING id, organization_id, user_id, role, status, created_at, updated_at`,
		memID, orgID, ownerUserID,
	).Scan(&mem.ID, &mem.OrganizationID, &mem.UserID, &mem.Role, &mem.Status, &mem.CreatedAt, &mem.UpdatedAt)
	if err != nil {
		return Organization{}, Membership{}, err
	}

	if err := tx.Commit(ctx); err != nil {
		return Organization{}, Membership{}, err
	}
	return org, mem, nil
}

func (r *PGXRepository) GetByID(ctx context.Context, orgID string) (Organization, error) {
	var o Organization
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, slug, status, created_at, updated_at FROM organization.organizations WHERE id = $1`,
		orgID,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, ErrNotFound
	}
	return o, err
}

func (r *PGXRepository) Update(ctx context.Context, orgID, name string) (Organization, error) {
	var o Organization
	err := r.pool.QueryRow(ctx,
		`UPDATE organization.organizations SET name = $2, updated_at = now() WHERE id = $1
		 RETURNING id, name, slug, status, created_at, updated_at`,
		orgID, name,
	).Scan(&o.ID, &o.Name, &o.Slug, &o.Status, &o.CreatedAt, &o.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Organization{}, ErrNotFound
	}
	return o, err
}

func (r *PGXRepository) GetMembership(ctx context.Context, orgID, userID string) (Membership, error) {
	var m Membership
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, user_id, role, status, created_at, updated_at
		 FROM organization.memberships WHERE organization_id = $1 AND user_id = $2`,
		orgID, userID,
	).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrNotFound
	}
	return m, err
}

func (r *PGXRepository) ListMembers(ctx context.Context, orgID string) ([]Membership, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, user_id, role, status, created_at, updated_at
		 FROM organization.memberships WHERE organization_id = $1 AND status = 'active'
		 ORDER BY created_at ASC`,
		orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Membership
	for rows.Next() {
		var m Membership
		if err := rows.Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (r *PGXRepository) beginOrReuse(ctx context.Context) (pgx.Tx, bool, error) {
	if t, ok := tx.TxFromContext(ctx); ok {
		return t, false, nil
	}
	t, err := r.pool.Begin(ctx)
	return t, true, err
}

func (r *PGXRepository) ChangeMemberRole(ctx context.Context, orgID, targetUserID string, newRole Role) (Membership, error) {
	t, owned, err := r.beginOrReuse(ctx)
	if err != nil {
		return Membership{}, err
	}
	if owned {
		defer func() { _ = t.Rollback(ctx) }()
	}

	var currentRole Role
	err = t.QueryRow(ctx,
		`SELECT role FROM organization.memberships WHERE organization_id = $1 AND user_id = $2 AND status = 'active' FOR UPDATE`,
		orgID, targetUserID,
	).Scan(&currentRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return Membership{}, ErrNotFound
	}
	if err != nil {
		return Membership{}, err
	}
	if currentRole == RoleOwner && newRole != RoleOwner {
		return Membership{}, ErrLastOwner
	}

	var m Membership
	err = t.QueryRow(ctx,
		`UPDATE organization.memberships SET role = $3, updated_at = now()
		 WHERE organization_id = $1 AND user_id = $2 AND status = 'active'
		 RETURNING id, organization_id, user_id, role, status, created_at, updated_at`,
		orgID, targetUserID, string(newRole),
	).Scan(&m.ID, &m.OrganizationID, &m.UserID, &m.Role, &m.Status, &m.CreatedAt, &m.UpdatedAt)
	if err != nil {
		return Membership{}, err
	}
	if owned {
		if err := t.Commit(ctx); err != nil {
			return Membership{}, err
		}
	}
	return m, nil
}

func (r *PGXRepository) RemoveMember(ctx context.Context, orgID, targetUserID string) error {
	t, owned, err := r.beginOrReuse(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer func() { _ = t.Rollback(ctx) }()
	}

	var currentRole Role
	err = t.QueryRow(ctx,
		`SELECT role FROM organization.memberships WHERE organization_id = $1 AND user_id = $2 AND status = 'active' FOR UPDATE`,
		orgID, targetUserID,
	).Scan(&currentRole)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if currentRole == RoleOwner {
		return ErrLastOwner
	}

	_, err = t.Exec(ctx,
		`UPDATE organization.memberships SET status = 'inactive', updated_at = now()
		 WHERE organization_id = $1 AND user_id = $2`,
		orgID, targetUserID,
	)
	if err != nil {
		return err
	}
	if owned {
		return t.Commit(ctx)
	}
	return nil
}

func (r *PGXRepository) TransferOwnership(ctx context.Context, orgID, currentOwnerID, newOwnerID string) error {
	t, owned, err := r.beginOrReuse(ctx)
	if err != nil {
		return err
	}
	if owned {
		defer func() { _ = t.Rollback(ctx) }()
	}

	var ownerRole Role
	err = t.QueryRow(ctx,
		`SELECT role FROM organization.memberships WHERE organization_id = $1 AND user_id = $2 AND status = 'active' FOR UPDATE`,
		orgID, currentOwnerID,
	).Scan(&ownerRole)
	if err != nil || ownerRole != RoleOwner {
		return ErrForbidden
	}

	var targetStatus MembershipStatus
	err = t.QueryRow(ctx,
		`SELECT status FROM organization.memberships WHERE organization_id = $1 AND user_id = $2 FOR UPDATE`,
		orgID, newOwnerID,
	).Scan(&targetStatus)
	if errors.Is(err, pgx.ErrNoRows) || targetStatus != MembershipActive {
		return ErrNotFound
	}
	if err != nil {
		return err
	}

	if _, err := t.Exec(ctx,
		`UPDATE organization.memberships SET role = 'admin', updated_at = now()
		 WHERE organization_id = $1 AND user_id = $2`,
		orgID, currentOwnerID,
	); err != nil {
		return err
	}

	if _, err := t.Exec(ctx,
		`UPDATE organization.memberships SET role = 'owner', updated_at = now()
		 WHERE organization_id = $1 AND user_id = $2`,
		orgID, newOwnerID,
	); err != nil {
		return err
	}

	if owned {
		return t.Commit(ctx)
	}
	return nil
}
