package knowledge

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"
)

type EntityType string

const (
	EntityCountry          EntityType = "country"
	EntityRegion           EntityType = "region"
	EntityProducer         EntityType = "producer"
	EntitySpecies          EntityType = "species"
	EntityVariety          EntityType = "variety"
	EntityProcessingMethod EntityType = "processing_method"
	EntityRoastLevel       EntityType = "roast_level"
	EntityFlavorDescriptor EntityType = "flavor_descriptor"
)

func (e EntityType) Valid() bool {
	switch e {
	case EntityCountry, EntityRegion, EntityProducer, EntitySpecies,
		EntityVariety, EntityProcessingMethod, EntityRoastLevel, EntityFlavorDescriptor:
		return true
	default:
		return false
	}
}

type Locale string

const (
	LocaleID Locale = "id"
	LocaleEN Locale = "en"
)

func (l Locale) Valid() bool {
	return l == LocaleID || l == LocaleEN
}

type Confidence string

const (
	ConfidenceHigh        Confidence = "high"
	ConfidenceMedium      Confidence = "medium"
	ConfidenceExploratory Confidence = "exploratory"
)

var (
	ErrNotFound     = errors.New("knowledge entity not found")
	ErrConflict     = errors.New("knowledge entity already exists")
	ErrInvalidInput = errors.New("invalid knowledge input")
	ErrForeignKey   = errors.New("referenced knowledge entity does not exist")
	validSlugRe     = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}[a-z0-9]$`)
	validISOCodeRe  = regexp.MustCompile(`^[A-Z]{2}$`)
)

func ValidateSlug(slug string) bool {
	return validSlugRe.MatchString(slug)
}

func ValidateISOCode(code string) bool {
	return validISOCodeRe.MatchString(code)
}

type Country struct {
	ID        string
	ISOCode   string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Region struct {
	ID        string
	CountryID string
	ParentID  *string
	Slug      string
	MinMASL   *int
	MaxMASL   *int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Producer struct {
	ID           string
	RegionID     string
	ProducerType string
	Slug         string
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type Species struct {
	ID             string
	ScientificName string
	Slug           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type Variety struct {
	ID        string
	SpeciesID string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProcessingMethod struct {
	ID        string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type RoastLevel struct {
	ID        string
	Slug      string
	SortOrder int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type FlavorDescriptor struct {
	ID        string
	ParentID  *string
	Slug      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Translation struct {
	ID         string
	EntityType EntityType
	EntityID   string
	Locale     Locale
	Title      string
	Summary    string
	Body       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func ValidateTranslation(t Translation) error {
	if !t.EntityType.Valid() || !t.Locale.Valid() {
		return ErrInvalidInput
	}
	if strings.TrimSpace(t.EntityID) == "" || strings.TrimSpace(t.Title) == "" {
		return ErrInvalidInput
	}
	return nil
}

type Repository interface {
	CreateCountry(ctx context.Context, isoCode, slug string) (Country, error)
	CreateRegion(ctx context.Context, countryID string, parentID *string, slug string, minMASL, maxMASL *int) (Region, error)
	CreateProducer(ctx context.Context, regionID, producerType, slug string) (Producer, error)
	CreateSpecies(ctx context.Context, scientificName, slug string) (Species, error)
	CreateVariety(ctx context.Context, speciesID, slug string) (Variety, error)
	CreateProcessingMethod(ctx context.Context, slug string) (ProcessingMethod, error)
	CreateRoastLevel(ctx context.Context, slug string, sortOrder int) (RoastLevel, error)
	CreateFlavorDescriptor(ctx context.Context, parentID *string, slug string) (FlavorDescriptor, error)
	LinkRegionVariety(ctx context.Context, regionID, varietyID string, conf Confidence) error
	LinkVarietyFlavor(ctx context.Context, varietyID, flavorID string, conf Confidence) error
	LinkProcessFlavor(ctx context.Context, processID, flavorID string, conf Confidence) error
	InsertTranslation(ctx context.Context, t Translation) (Translation, error)
	DeleteCountry(ctx context.Context, id string) error
}
