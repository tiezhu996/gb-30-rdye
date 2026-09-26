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

var activeStatuses = []string{
	constants.AppStatusSubmitted,
	constants.AppStatusOrgReview,
	constants.AppStatusCommunicating,
	constants.AppStatusConfirmed,
	constants.AppStatusOfflineInterview,
}

// FindActiveByUserAndPet checks for an open (non-terminal) application
// from the same user for the same pet. Closed applications do not block
// the user from applying again.
func (r *AdoptionApplicationRepository) FindActiveByUserAndPet(userID, petID uint) (*model.AdoptionApplication, error) {
	var a model.AdoptionApplication
	if err := translate(r.db.Where("user_id = ? AND pet_id = ? AND status IN ?", userID, petID, activeStatuses).First(&a).Error); err != nil {
		return nil, err
	}
	return &a, nil
}

// CountActiveByPet returns the number of open applications for a pet.
func (r *AdoptionApplicationRepository) CountActiveByPet(petID uint) (int64, error) {
	var n int64
	if err := r.db.Model(&model.AdoptionApplication{}).
		Where("pet_id = ? AND status IN ?", petID, activeStatuses).Count(&n).Error; err != nil {
		return 0, err
	}
	return n, nil
}
