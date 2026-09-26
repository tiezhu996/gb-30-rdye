package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
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
	// Only an open application blocks re-applying; rejected/withdrawn
	// applications leave the pet available and the same user may apply again.
	if exist, err := s.repo.FindActiveByUserAndPet(userID, petID); err == nil && exist != nil {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("Application[user_id=%d pet_id=%d] submit failed: already applied", userID, petID))
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
	// Home recommendations no longer include this pet.
	s.invalidateHomeCache()
	s.logger.Info(fmt.Sprintf(constants.LogAppSubmitSuccess, a.ID, petID), "id", a.ID)
	return a, nil
}

// UpdateStatus transitions an application along the state machine.
//
// Org-side transitions (advance/reject) are only allowed for the org that
// owns the pet; the applicant may only withdraw an open application.
// Rejection and withdrawal release the pet back to available, while approval
// marks the pet adopted. Once approved the application is terminal and the
// record can no longer be changed.
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
	if constants.IsTerminalApplicationStatus(a.Status) {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("AdoptionApplication[id=%d] status change failed: terminal status=%s is locked", id, a.Status))
	}
	// Authorize the requested transition.
	switch role {
	case "org":
		org, err := s.orgRepo.FindByUserID(userID)
		if err != nil || org.ID != a.OrgID {
			return nil, util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("AdoptionApplication[id=%d] status change failed: user_id=%d not org owner", id, userID))
		}
		if next == constants.AppStatusWithdrawn {
			return nil, util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("AdoptionApplication[id=%d] status change failed: only the applicant can withdraw", id))
		}
	case "user":
		if a.UserID != userID {
			return nil, util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("AdoptionApplication[id=%d] status change failed: not owner", id))
		}
		if next != constants.AppStatusWithdrawn {
			return nil, util.NewAppError(403, constants.CodeForbidden,
				fmt.Sprintf("AdoptionApplication[id=%d] status change failed: applicant may only withdraw", id))
		}
	default:
		return nil, util.NewAppError(403, constants.CodeForbidden,
			fmt.Sprintf("AdoptionApplication[id=%d] status change failed: role=%s not allowed", id, role))
	}
	allowed := false
	for _, s2 := range constants.NextApplicationStatuses(a.Status) {
		if s2 == next {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, util.NewAppError(409, constants.CodeConflict,
			fmt.Sprintf("AdoptionApplication[id=%d] status change failed: %s -> %s not allowed", id, a.Status, next))
	}

	a.Status = next
	if next == constants.AppStatusRejected || next == constants.AppStatusWithdrawn {
		a.CloseReason = normalizeCloseReason(next, reason)
	}

	pet, err := s.petRepo.FindByID(a.PetID)
	if err != nil {
		return nil, fmt.Errorf("application status pet find: %w", err)
	}

	// Approval and release both update the pet atomically with the application.
	var petNext string
	switch next {
	case constants.AppStatusApproved:
		petNext = constants.PetStatusAdopted
	case constants.AppStatusRejected, constants.AppStatusWithdrawn:
		petNext = constants.PetStatusAvailable
	}

	if petNext != "" {
		// Defensive: never release a pet that still has another open application.
		if petNext == constants.PetStatusAvailable {
			open, err := s.repo.CountActiveByPet(a.PetID)
			if err != nil {
				return nil, fmt.Errorf("application status open count: %w", err)
			}
			// This application is still in an active status until saved,
			// so the count must be exactly 1 (itself).
			if open > 1 {
				return nil, util.NewAppError(409, constants.CodeConflict,
					fmt.Sprintf("AdoptionApplication[id=%d] close failed: pet_id=%d still has open applications", id, a.PetID))
			}
		}
		pet.Status = petNext
		err = s.db.Transaction(func(tx *gorm.DB) error {
			if err := s.repo.UpdateTx(tx, a); err != nil {
				return fmt.Errorf("application status update: %w", err)
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
	} else {
		if err := s.repo.Update(a); err != nil {
			s.logger.Error(fmt.Sprintf(constants.LogAppStatusChangeFailed, id), "error", err)
			return nil, fmt.Errorf("application status update: %w", err)
		}
	}

	// Pet availability changed (submit/approval/release): refresh the home
	// recommendation cache so it reflects current availability immediately.
	if petNext != "" {
		s.invalidateHomeCache()
	}
	s.logger.Info(fmt.Sprintf(constants.LogAppStatusChanged, id, next), "id", id)
	return a, nil
}

// invalidateHomeCache drops the aggregated home overview so the next request
// rebuilds it with current pet availability. Failures are non-fatal (TTL).
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

// normalizeCloseReason fills a readable default when no reason is supplied.
func normalizeCloseReason(status, reason string) string {
	if reason != "" {
		return reason
	}
	if status == constants.AppStatusWithdrawn {
		return "申请人主动撤回申请"
	}
	return "机构审核未通过"
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
	items, err := s.repo.ListByOrg(org.ID, status)
	if err != nil {
		return nil, fmt.Errorf("application list by org: %w", err)
	}
	return items, nil
}
