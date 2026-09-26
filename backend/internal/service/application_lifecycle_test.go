package service

import (
	"io"
	"log/slog"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"

	"github.com/gbadopt/gbadopt/internal/constants"
	"github.com/gbadopt/gbadopt/internal/model"
	"github.com/gbadopt/gbadopt/internal/repository"
)

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func newTestAppService(t *testing.T) (*ApplicationService, *gorm.DB, uint, uint, uint, uint) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&model.User{}, &model.Organization{}, &model.Pet{}, &model.AdoptionApplication{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	orgUser := &model.User{Username: "org", Email: "org@test.local", Role: "org"}
	adopter := &model.User{Username: "adopter", Email: "adopter@test.local", Role: "user"}
	other := &model.User{Username: "other", Email: "other@test.local", Role: "user"}
	for _, u := range []*model.User{orgUser, adopter, other} {
		if err := db.Create(u).Error; err != nil {
			t.Fatalf("create user: %v", err)
		}
	}
	org := &model.Organization{UserID: orgUser.ID, Name: "shelter", Status: constants.OrgStatusApproved}
	if err := db.Create(org).Error; err != nil {
		t.Fatalf("create org: %v", err)
	}
	pet := &model.Pet{OrgID: org.ID, Name: "wang", Species: constants.PetSpeciesDog, Status: constants.PetStatusAvailable}
	if err := db.Create(pet).Error; err != nil {
		t.Fatalf("create pet: %v", err)
	}

	svc := NewApplicationService(
		db,
		repository.NewAdoptionApplicationRepository(db),
		repository.NewPetRepository(db),
		repository.NewOrganizationRepository(db),
		nil,
		testLogger(),
	)
	return svc, db, orgUser.ID, adopter.ID, other.ID, pet.ID
}

func TestApplicationLifecycle_RejectReleasesPet(t *testing.T) {
	svc, db, orgUserID, adopterID, otherID, petID := newTestAppService(t)

	a, err := svc.Submit(adopterID, petID, "{}")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := petStatus(t, db, petID); got != constants.PetStatusPending {
		t.Fatalf("pet status after submit = %s, want pending", got)
	}

	// another adopter cannot apply while an application is open.
	if _, err := svc.Submit(otherID, petID, "{}"); err == nil {
		t.Fatal("expected conflict for second applicant, got nil")
	}

	if _, err := svc.UpdateStatus(orgUserID, a.ID, "org", constants.AppStatusRejected, "条件不符"); err != nil {
		t.Fatalf("reject: %v", err)
	}
	if got := petStatus(t, db, petID); got != constants.PetStatusAvailable {
		t.Fatalf("pet status after reject = %s, want available", got)
	}
	var rejected model.AdoptionApplication
	db.First(&rejected, a.ID)
	if rejected.Status != constants.AppStatusRejected || rejected.CloseReason != "条件不符" {
		t.Fatalf("rejected record = status=%s reason=%q", rejected.Status, rejected.CloseReason)
	}

	// pet reapplies are allowed after rejection (same user and others).
	if _, err := svc.Submit(adopterID, petID, "{}"); err != nil {
		t.Fatalf("same user re-apply after reject failed: %v", err)
	}

	// the terminal record cannot be changed anymore.
	if _, err := svc.UpdateStatus(orgUserID, a.ID, "org", constants.AppStatusOrgReview, ""); err == nil {
		t.Fatal("expected error modifying rejected application")
	}
}

func TestApplicationLifecycle_WithdrawReleasesPet(t *testing.T) {
	svc, db, orgUserID, adopterID, _, petID := newTestAppService(t)

	a, err := svc.Submit(adopterID, petID, "")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}

	// org cannot withdraw an applicant's application.
	if _, err := svc.UpdateStatus(orgUserID, a.ID, "org", constants.AppStatusWithdrawn, ""); err == nil {
		t.Fatal("expected org withdrawal to be forbidden")
	}
	// applicant cannot self-approve.
	if _, err := svc.UpdateStatus(adopterID, a.ID, "user", constants.AppStatusApproved, ""); err == nil {
		t.Fatal("expected applicant approval to be forbidden")
	}

	if _, err := svc.UpdateStatus(adopterID, a.ID, "user", constants.AppStatusWithdrawn, ""); err != nil {
		t.Fatalf("withdraw: %v", err)
	}
	if got := petStatus(t, db, petID); got != constants.PetStatusAvailable {
		t.Fatalf("pet status after withdraw = %s, want available", got)
	}
	var w model.AdoptionApplication
	db.First(&w, a.ID)
	if w.Status != constants.AppStatusWithdrawn || w.CloseReason == "" {
		t.Fatalf("withdrawn record = status=%s reason=%q", w.Status, w.CloseReason)
	}
}

func TestApplicationLifecycle_ApproveLocksRecordAndAdoptsPet(t *testing.T) {
	svc, db, orgUserID, adopterID, otherID, petID := newTestAppService(t)

	a, err := svc.Submit(adopterID, petID, "")
	if err != nil {
		t.Fatalf("submit: %v", err)
	}
	chain := []string{
		constants.AppStatusOrgReview,
		constants.AppStatusCommunicating,
		constants.AppStatusConfirmed,
		constants.AppStatusOfflineInterview,
		constants.AppStatusApproved,
	}
	for _, next := range chain {
		a, err = svc.UpdateStatus(orgUserID, a.ID, "org", next, "")
		if err != nil {
			t.Fatalf("advance to %s: %v", next, err)
		}
	}
	if got := petStatus(t, db, petID); got != constants.PetStatusAdopted {
		t.Fatalf("pet status after approve = %s, want adopted", got)
	}
	// applicant withdraw after approval must fail.
	if _, err := svc.UpdateStatus(adopterID, a.ID, "user", constants.AppStatusWithdrawn, ""); err == nil {
		t.Fatal("expected withdrawal after approval to fail")
	}
	// adopted pet cannot receive new applications.
	if _, err := svc.Submit(otherID, petID, "{}"); err == nil {
		t.Fatal("expected application on adopted pet to fail")
	}
}

func petStatus(t *testing.T, db *gorm.DB, petID uint) string {
	t.Helper()
	var p model.Pet
	if err := db.First(&p, petID).Error; err != nil {
		t.Fatalf("find pet: %v", err)
	}
	return p.Status
}
