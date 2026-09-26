package repository

import (
	"gorm.io/gorm"

	"github.com/gbadopt/gbadopt/internal/constants"
	"github.com/gbadopt/gbadopt/internal/model"
)

// AdoptionApplicationRepository handles application persistence.
type AdoptionApplicationRepository struct{ db *gorm.DB }

// NewAdoptionApplicationRepository creates the repository.
func NewAdoptionApplicationRepository(db *gorm.DB) *AdoptionApplicationRepository {
	return &AdoptionApplicationRepository{db: db}
}

// Create inserts an application.
func (r *AdoptionApplicationRepository) Create(a *model.AdoptionApplication) error {
	return translate(r.db.Create(a).Error)
}

// CreateTx inserts an application within an outer transaction.
func (r *AdoptionApplicationRepository) CreateTx(tx *gorm.DB, a *model.AdoptionApplication) error {
	return translate(tx.Create(a).Error)
}

// FindByID locates an application by id.
func (r *AdoptionApplicationRepository) FindByID(id uint) (*model.AdoptionApplication, error) {
	var a model.AdoptionApplication
	if err := translate(r.db.First(&a, id).Error); err != nil {
		return nil, err
	}
	return &a, nil
}

// Update persists an application.
func (r *AdoptionApplicationRepository) Update(a *model.AdoptionApplication) error {
	return translate(r.db.Save(a).Error)
}

// UpdateTx persists an application within an outer transaction.
func (r *AdoptionApplicationRepository) UpdateTx(tx *gorm.DB, a *model.AdoptionApplication) error {
	return translate(tx.Save(a).Error)
}

// ListByUser returns applications of a user.
func (r *AdoptionApplicationRepository) ListByUser(userID uint) ([]model.AdoptionApplication, error) {
	var items []model.AdoptionApplication
	if err := r.db.Where("user_id = ?", userID).Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// ListByOrg returns applications targeting an org.
func (r *AdoptionApplicationRepository) ListByOrg(orgID uint, status string) ([]model.AdoptionApplication, error) {
	var items []model.AdoptionApplication
	q := r.db.Where("org_id = ?", orgID)
	if status != "" {
		q = q.Where("status = ?", status)
	}
	if err := q.Order("id DESC").Find(&items).Error; err != nil {
		return nil, err
	}
	return items, nil
}

// FindActiveByUserAndPet returns the user's in-progress application for the same pet.
// Closed applications (rejected/withdrawn/approved) are intentionally ignored so an
// applicant may re-apply after the pet becomes available again.
func (r *AdoptionApplicationRepository) FindActiveByUserAndPet(userID, petID uint) (*model.AdoptionApplication, error) {
	var a model.AdoptionApplication
	err := translate(
		r.db.Where("user_id = ? AND pet_id = ? AND status IN ?", userID, petID, constants.ActiveApplicationStatuses()).
			First(&a).Error)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

// CountActiveByPetIDTx counts in-progress applications of a pet within a transaction.
// It is used when an application closes: the pet goes back to "available" only when
// no other active application is competing for it.
func (r *AdoptionApplicationRepository) CountActiveByPetIDTx(tx *gorm.DB, petID uint, excludeAppID uint) (int64, error) {
	var n int64
	err := tx.Model(&model.AdoptionApplication{}).
		Where("pet_id = ? AND id <> ? AND status IN ?", petID, excludeAppID, constants.ActiveApplicationStatuses()).
		Count(&n).Error
	return n, err
}
