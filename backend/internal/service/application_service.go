package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/gbadopt/gbadopt/internal/constants"
	"github.com/gbadopt/gbadopt/internal/model"
	"github.com/gbadopt/gbadopt/internal/repository"
	"github.com/gbadopt/gbadopt/internal/util"
)

// ApplicationService implements the adoption application state machine.
type ApplicationService struct {
	db      *gorm.DB
	repo    *repository.AdoptionApplicationRepository
	petRepo *repository.PetRepository
	orgRepo *repository.OrganizationRepository
	redis   *util.RedisClient
	logger  *slog.Logger
}

// NewApplicationService creates an ApplicationService.
func NewApplicationService(db *gorm.DB, repo *repository.AdoptionApplicationRepository, petRepo *repository.PetRepository, orgRepo *repository.OrganizationRepository, redis *util.RedisClient, logger *slog.Logger) *ApplicationService {
	return &ApplicationService{db: db, repo: repo, petRepo: petRepo, orgRepo: orgRepo, redis: redis, logger: logger}
}

// invalidateHomeCache drops the home overview cache so availability changes are visible immediately.
// Failures are non-fatal: the cache also expires via its short TTL.
func (s *ApplicationService) invalidateHomeCache() {
	if s.redis == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err := s.redis.Delete(ctx, constants.CacheKeyHomeOverview); err != nil {
		s.logger.Warn("home overview cache invalidate failed", "error", err)
	}
}

// Submit creates an application from a user to a pet.
func (s *ApplicationService) Submit(userID, petID uint, questionnaire string) (*model.AdoptionApplication, error) {
	pet, err := s.petRepo.FindByID(petID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("Pet[id=%d] not found", petID))
		}
		return nil, fmt.Errorf("application submit pet find: %w", err)
	}
	if pet.Status != constants.PetStatusAvailable {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("Application[pet_id=%d] submit failed: pet not available (status=%s)", petID, pet.Status))
	}
	// Only an in-progress application blocks re-applying; rejected/withdrawn history stays visible
	// but lets the same user apply again once the pet is available.
	if exist, err := s.repo.FindActiveByUserAndPet(userID, petID); err == nil && exist != nil {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("Application[user_id=%d pet_id=%d] submit failed: active application id=%d", userID, petID, exist.ID))
	}
	a := &model.AdoptionApplication{
		UserID: userID, PetID: petID, OrgID: pet.OrgID,
		Questionnaire: questionnaire, Status: constants.AppStatusSubmitted,
	}
	if a.Questionnaire == "" {
		a.Questionnaire = "{}"
	}
	// pet becomes pending atomically with the application creation.
	pet.Status = constants.PetStatusPending
	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.CreateTx(tx, a); err != nil {
			return fmt.Errorf("application submit: %w", err)
		}
		if err := s.petRepo.UpdateTx(tx, pet); err != nil {
			return fmt.Errorf("application submit pet update: %w", err)
		}
		return nil
	})
	if err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogAppSubmitFailed, petID), "error", err)
		return nil, err
	}
	s.invalidateHomeCache()
	s.logger.Info(fmt.Sprintf(constants.LogAppSubmitSuccess, a.ID, petID), "id", a.ID)
	return a, nil
}

// UpdateStatus transitions an application along the state machine.
//
//   - org: advances the review pipeline, or rejects (reason required) at any active stage.
//   - user (applicant): withdraws (reason required) while the application is active.
//   - admin: not allowed to mutate applications.
//
// Pet side effects run in the same transaction: approved -> pet adopted (immutable);
// rejected/withdrawn -> pet back to available when no other active application remains.
// Terminal applications (approved/rejected/withdrawn) can never be changed again.
func (s *ApplicationService) UpdateStatus(userID, id uint, role string, next, reason string) (*model.AdoptionApplication, error) {
	if !constants.IsValidApplicationStatus(next) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("Application[id=%d] status=%s invalid", id, next))
	}
	a, err := s.repo.FindByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, util.NewAppError(404, constants.CodeNotFound, fmt.Sprintf("AdoptionApplication[id=%d] not found", id))
		}
		return nil, fmt.Errorf("application status find: %w", err)
	}
	reason = strings.TrimSpace(reason)

	// Authorization + transition validation is role-specific.
	switch role {
	case "org":
		org, err := s.orgRepo.FindByUserID(userID)
		if err != nil || org.ID != a.OrgID {
			return nil, util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("AdoptionApplication[id=%d] status change failed: user_id=%d not org owner", id, userID))
		}
		if !constants.CanOrgTransition(a.Status, next) {
			return nil, util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("AdoptionApplication[id=%d] status change failed: %s -> %s not allowed for org", id, a.Status, next))
		}
		if next == constants.AppStatusRejected && reason == "" {
			return nil, util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("AdoptionApplication[id=%d] reject failed: reason is required", id))
		}
	case "user":
		if a.UserID != userID {
			return nil, util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("AdoptionApplication[id=%d] status change failed: not owner", id))
		}
		if next != constants.AppStatusWithdrawn || !constants.CanUserWithdraw(a.Status) {
			return nil, util.NewAppError(409, constants.CodeConflict,
				fmt.Sprintf("AdoptionApplication[id=%d] status change failed: %s -> %s not allowed for user", id, a.Status, next))
		}
		if reason == "" {
			return nil, util.NewAppError(422, constants.CodeValidationError,
				fmt.Sprintf("AdoptionApplication[id=%d] withdraw failed: reason is required", id))
		}
	default:
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("AdoptionApplication[id=%d] status change failed: role=%s not allowed", id, role))
	}

	// Load the pet up front; every branch below updates it inside the transaction.
	pet, err := s.petRepo.FindByID(a.PetID)
	if err != nil {
		return nil, fmt.Errorf("application status pet find: %w", err)
	}

	a.Status = next
	if next == constants.AppStatusRejected || next == constants.AppStatusWithdrawn {
		a.CloseReason = reason
	}

	switch next {
	case constants.AppStatusApproved:
		// Adoption is final: the pet is locked as adopted and this record never changes again.
		pet.Status = constants.PetStatusAdopted
	case constants.AppStatusRejected, constants.AppStatusWithdrawn:
		// The pet becomes adoptable again only if no competing active application remains.
		pet.Status = constants.PetStatusAvailable
	}

	err = s.db.Transaction(func(tx *gorm.DB) error {
		if err := s.repo.UpdateTx(tx, a); err != nil {
			return fmt.Errorf("application status update: %w", err)
		}
		if next == constants.AppStatusRejected || next == constants.AppStatusWithdrawn {
			otherActive, err := s.repo.CountActiveByPetIDTx(tx, a.PetID, a.ID)
			if err != nil {
				return fmt.Errorf("application close active count: %w", err)
			}
			if otherActive > 0 {
				// Another application is still being processed; keep the pet pending.
				pet.Status = constants.PetStatusPending
			}
		}
		if err := s.petRepo.UpdateTx(tx, pet); err != nil {
			return fmt.Errorf("application status pet update: %w", err)
		}
		return nil
	})
	if err != nil {
		s.logger.Error(fmt.Sprintf(constants.LogAppStatusChangeFailed, id), "error", err)
		return nil, err
	}
	if pet.Status != constants.PetStatusPending {
		// Pet availability changed (adopted or released) -> refresh home recommendations at once.
		s.invalidateHomeCache()
	}
	s.logger.Info(fmt.Sprintf(constants.LogAppStatusChanged, id, next), "id", id)
	return a, nil
}

// ListByUser returns a user's applications.
func (s *ApplicationService) ListByUser(userID uint) ([]model.AdoptionApplication, error) {
	items, err := s.repo.ListByUser(userID)
	if err != nil {
		return nil, fmt.Errorf("application list by user: %w", err)
	}
	return items, nil
}

// ListByOrg returns applications for an org.
func (s *ApplicationService) ListByOrg(userID uint, status string) ([]model.AdoptionApplication, error) {
	org, err := s.orgRepo.FindByUserID(userID)
	if err != nil {
		return nil, util.NewAppError(403, constants.CodeForbidden, "org profile not found")
	}
	if status != "" && !constants.IsValidApplicationStatus(status) {
		return nil, util.NewAppError(422, constants.CodeValidationError,
			fmt.Sprintf("application list by org: status=%s invalid", status))
	}
	items, err := s.repo.ListByOrg(org.ID, status)
	if err != nil {
		return nil, fmt.Errorf("application list by org: %w", err)
	}
	return items, nil
}
