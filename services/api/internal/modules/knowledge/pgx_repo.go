package knowledge

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type PGXRepository struct {
	pool *pgxpool.Pool
}

func NewPGXRepository(pool *pgxpool.Pool) *PGXRepository {
	return &PGXRepository{pool: pool}
}

func mapPGError(err error) error {
	if err == nil {
		return nil
	}
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505":
			return ErrConflict
		case "23503":
			return ErrForeignKey
		case "23514", "P0001":
			return ErrInvalidInput
		}
	}
	return err
}

func (r *PGXRepository) CreateCountry(ctx context.Context, isoCode, slug string) (Country, error) {
	var c Country
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.countries (id, iso_code, slug) VALUES ($1, $2, $3)
		 RETURNING id, iso_code, slug, created_at, updated_at`,
		uuid.NewString(), isoCode, slug,
	).Scan(&c.ID, &c.ISOCode, &c.Slug, &c.CreatedAt, &c.UpdatedAt)
	return c, mapPGError(err)
}

func (r *PGXRepository) CreateRegion(ctx context.Context, countryID string, parentID *string, slug string, minMASL, maxMASL *int) (Region, error) {
	var reg Region
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.regions (id, country_id, parent_id, slug, min_masl, max_masl)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, country_id, parent_id, slug, min_masl, max_masl, created_at, updated_at`,
		uuid.NewString(), countryID, parentID, slug, minMASL, maxMASL,
	).Scan(&reg.ID, &reg.CountryID, &reg.ParentID, &reg.Slug, &reg.MinMASL, &reg.MaxMASL, &reg.CreatedAt, &reg.UpdatedAt)
	return reg, mapPGError(err)
}

func (r *PGXRepository) CreateProducer(ctx context.Context, regionID, producerType, slug string) (Producer, error) {
	var p Producer
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.producers (id, region_id, producer_type, slug)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id, region_id, producer_type, slug, created_at, updated_at`,
		uuid.NewString(), regionID, producerType, slug,
	).Scan(&p.ID, &p.RegionID, &p.ProducerType, &p.Slug, &p.CreatedAt, &p.UpdatedAt)
	return p, mapPGError(err)
}

func (r *PGXRepository) CreateSpecies(ctx context.Context, scientificName, slug string) (Species, error) {
	var s Species
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.species (id, scientific_name, slug)
		 VALUES ($1, $2, $3)
		 RETURNING id, scientific_name, slug, created_at, updated_at`,
		uuid.NewString(), scientificName, slug,
	).Scan(&s.ID, &s.ScientificName, &s.Slug, &s.CreatedAt, &s.UpdatedAt)
	return s, mapPGError(err)
}

func (r *PGXRepository) CreateVariety(ctx context.Context, speciesID, slug string) (Variety, error) {
	var v Variety
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.varieties (id, species_id, slug)
		 VALUES ($1, $2, $3)
		 RETURNING id, species_id, slug, created_at, updated_at`,
		uuid.NewString(), speciesID, slug,
	).Scan(&v.ID, &v.SpeciesID, &v.Slug, &v.CreatedAt, &v.UpdatedAt)
	return v, mapPGError(err)
}

func (r *PGXRepository) CreateProcessingMethod(ctx context.Context, slug string) (ProcessingMethod, error) {
	var pm ProcessingMethod
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.processing_methods (id, slug) VALUES ($1, $2)
		 RETURNING id, slug, created_at, updated_at`,
		uuid.NewString(), slug,
	).Scan(&pm.ID, &pm.Slug, &pm.CreatedAt, &pm.UpdatedAt)
	return pm, mapPGError(err)
}

func (r *PGXRepository) CreateRoastLevel(ctx context.Context, slug string, sortOrder int) (RoastLevel, error) {
	var rl RoastLevel
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.roast_levels (id, slug, sort_order) VALUES ($1, $2, $3)
		 RETURNING id, slug, sort_order, created_at, updated_at`,
		uuid.NewString(), slug, sortOrder,
	).Scan(&rl.ID, &rl.Slug, &rl.SortOrder, &rl.CreatedAt, &rl.UpdatedAt)
	return rl, mapPGError(err)
}

func (r *PGXRepository) CreateFlavorDescriptor(ctx context.Context, parentID *string, slug string) (FlavorDescriptor, error) {
	var fd FlavorDescriptor
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.flavor_descriptors (id, parent_id, slug) VALUES ($1, $2, $3)
		 RETURNING id, parent_id, slug, created_at, updated_at`,
		uuid.NewString(), parentID, slug,
	).Scan(&fd.ID, &fd.ParentID, &fd.Slug, &fd.CreatedAt, &fd.UpdatedAt)
	return fd, mapPGError(err)
}

func (r *PGXRepository) LinkRegionVariety(ctx context.Context, regionID, varietyID string, conf Confidence) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO knowledge.region_varieties (id, region_id, variety_id, confidence) VALUES ($1, $2, $3, $4)`,
		uuid.NewString(), regionID, varietyID, string(conf),
	)
	return mapPGError(err)
}

func (r *PGXRepository) LinkVarietyFlavor(ctx context.Context, varietyID, flavorID string, conf Confidence) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO knowledge.variety_flavors (id, variety_id, flavor_descriptor_id, confidence) VALUES ($1, $2, $3, $4)`,
		uuid.NewString(), varietyID, flavorID, string(conf),
	)
	return mapPGError(err)
}

func (r *PGXRepository) LinkProcessFlavor(ctx context.Context, processID, flavorID string, conf Confidence) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO knowledge.process_flavors (id, processing_method_id, flavor_descriptor_id, confidence) VALUES ($1, $2, $3, $4)`,
		uuid.NewString(), processID, flavorID, string(conf),
	)
	return mapPGError(err)
}

func (r *PGXRepository) InsertTranslation(ctx context.Context, t Translation) (Translation, error) {
	if err := ValidateTranslation(t); err != nil {
		return Translation{}, err
	}
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.Status == "" {
		t.Status = "draft"
	}
	var out Translation
	err := r.pool.QueryRow(ctx,
		`INSERT INTO knowledge.translations (id, entity_type, entity_id, locale, title, summary, body, status)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		 RETURNING id, entity_type, entity_id, locale, title, summary, body, status, created_at, updated_at`,
		t.ID, string(t.EntityType), t.EntityID, string(t.Locale), t.Title, t.Summary, t.Body, t.Status,
	).Scan(&out.ID, &out.EntityType, &out.EntityID, &out.Locale, &out.Title, &out.Summary, &out.Body, &out.Status, &out.CreatedAt, &out.UpdatedAt)
	return out, mapPGError(err)
}

func (r *PGXRepository) DeleteCountry(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM knowledge.countries WHERE id = $1`, id)
	return mapPGError(err)
}
