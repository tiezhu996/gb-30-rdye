package service

import (
	"database/sql"
	"log/slog"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"github.com/gbadopt/gbadopt/internal/constants"
	"github.com/gbadopt/gbadopt/internal/repository"
)

func newMockDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock, *sql.DB) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	gdb, err := gorm.Open(postgres.New(postgres.Config{Conn: sqlDB, WithoutReturning: true}), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	if err != nil {
		t.Fatalf("gorm open: %v", err)
	}
	return gdb, mock, sqlDB
}

func newAppService(gdb *gorm.DB) *ApplicationService {
	return NewApplicationService(
		gdb,
		repository.NewAdoptionApplicationRepository(gdb),
		repository.NewPetRepository(gdb),
		repository.NewOrganizationRepository(gdb),
		nil,
		slog.Default(),
	)
}

func TestUpdateStatus_RejectReleasesPet(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()

	svc := newAppService(gdb)
	const appID, petID, orgID, userID = uint(7), uint(9), uint(3), uint(5)

	// repo.FindByID
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "adoption_applications"`)).
		WithArgs(appID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "pet_id", "org_id", "status"}).
			AddRow(appID, userID, petID, orgID, constants.AppStatusSubmitted))
	// orgRepo.FindByUserID (role=org)
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(orgID, constants.OrgStatusApproved))
	// petRepo.FindByID
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pets"`)).
		WithArgs(petID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "org_id", "status"}).AddRow(petID, orgID, constants.PetStatusPending))

	mock.ExpectBegin()
	// update application (save)
	mock.ExpectExec(`UPDATE "adoption_applications"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	// count other active applications -> 0, so pet returns to available
	mock.ExpectQuery(regexp.QuoteMeta(`count(*)`)).
		WithArgs(
			petID, appID,
			constants.AppStatusSubmitted, constants.AppStatusOrgReview,
			constants.AppStatusCommunicating, constants.AppStatusConfirmed,
			constants.AppStatusOfflineInterview,
		).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`UPDATE "pets"`).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := svc.UpdateStatus(userID, appID, "org", constants.AppStatusRejected, "资料不完整")
	if err != nil {
		t.Fatalf("UpdateStatus reject: %v", err)
	}
	if got.Status != constants.AppStatusRejected || got.CloseReason != "资料不完整" {
		t.Errorf("application status/reason = %s/%q", got.Status, got.CloseReason)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestUpdateStatus_RejectWithoutReasonFails(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()
	svc := newAppService(gdb)
	const appID, petID, orgID, userID = uint(7), uint(9), uint(3), uint(5)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "adoption_applications"`)).
		WithArgs(appID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "pet_id", "org_id", "status"}).
			AddRow(appID, userID, petID, orgID, constants.AppStatusSubmitted))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(orgID, constants.OrgStatusApproved))

	if _, err := svc.UpdateStatus(userID, appID, "org", constants.AppStatusRejected, "  "); err == nil {
		t.Fatal("reject without reason must fail")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestUpdateStatus_WithdrawReleasesPet(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()
	svc := newAppService(gdb)
	const appID, petID, orgID, userID = uint(7), uint(9), uint(3), uint(5)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "adoption_applications"`)).
		WithArgs(appID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "pet_id", "org_id", "status"}).
			AddRow(appID, userID, petID, orgID, constants.AppStatusCommunicating))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pets"`)).
		WithArgs(petID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "org_id", "status"}).AddRow(petID, orgID, constants.PetStatusPending))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "adoption_applications"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`count(*)`)).
		WithArgs(
			petID, appID,
			constants.AppStatusSubmitted, constants.AppStatusOrgReview,
			constants.AppStatusCommunicating, constants.AppStatusConfirmed,
			constants.AppStatusOfflineInterview,
		).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec(`UPDATE "pets"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := svc.UpdateStatus(userID, appID, "user", constants.AppStatusWithdrawn, "家庭计划有变")
	if err != nil {
		t.Fatalf("UpdateStatus withdraw: %v", err)
	}
	if got.Status != constants.AppStatusWithdrawn {
		t.Errorf("status = %s, want withdrawn", got.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestUpdateStatus_WithdrawKeepsPendingWhenOtherActive(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()
	svc := newAppService(gdb)
	const appID, petID, orgID, userID = uint(7), uint(9), uint(3), uint(5)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "adoption_applications"`)).
		WithArgs(appID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "pet_id", "org_id", "status"}).
			AddRow(appID, userID, petID, orgID, constants.AppStatusSubmitted))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pets"`)).
		WithArgs(petID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "org_id", "status"}).AddRow(petID, orgID, constants.PetStatusPending))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "adoption_applications"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(regexp.QuoteMeta(`count(*)`)).
		WithArgs(
			petID, appID,
			constants.AppStatusSubmitted, constants.AppStatusOrgReview,
			constants.AppStatusCommunicating, constants.AppStatusConfirmed,
			constants.AppStatusOfflineInterview,
		).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1)) // another active application exists
	mock.ExpectExec(`UPDATE "pets"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := svc.UpdateStatus(userID, appID, "user", constants.AppStatusWithdrawn, "不想领养了")
	if err != nil {
		t.Fatalf("UpdateStatus: %v", err)
	}
	_ = got
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestUpdateStatus_ApproveAdoptsPet(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()
	svc := newAppService(gdb)
	const appID, petID, orgID, userID = uint(7), uint(9), uint(3), uint(5)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "adoption_applications"`)).
		WithArgs(appID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "pet_id", "org_id", "status"}).
			AddRow(appID, userID, petID, orgID, constants.AppStatusOfflineInterview))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(orgID, constants.OrgStatusApproved))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "pets"`)).
		WithArgs(petID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "org_id", "status"}).AddRow(petID, orgID, constants.PetStatusPending))

	mock.ExpectBegin()
	mock.ExpectExec(`UPDATE "adoption_applications"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`UPDATE "pets"`).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()

	got, err := svc.UpdateStatus(userID, appID, "org", constants.AppStatusApproved, "")
	if err != nil {
		t.Fatalf("approve: %v", err)
	}
	if got.Status != constants.AppStatusApproved {
		t.Errorf("status = %s", got.Status)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestUpdateStatus_TerminalIsImmutable(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()
	svc := newAppService(gdb)
	const appID, petID, orgID, userID = uint(7), uint(9), uint(3), uint(5)

	// approved record must never change again, not even by its org.
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "adoption_applications"`)).
		WithArgs(appID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "pet_id", "org_id", "status"}).
			AddRow(appID, userID, petID, orgID, constants.AppStatusApproved))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "organizations"`)).
		WithArgs(userID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}).AddRow(orgID, constants.OrgStatusApproved))

	if _, err := svc.UpdateStatus(userID, appID, "org", constants.AppStatusRejected, "x"); err == nil {
		t.Fatal("approved application must be immutable")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}

func TestUpdateStatus_UserCannotReject(t *testing.T) {
	gdb, mock, sqlDB := newMockDB(t)
	defer sqlDB.Close()
	svc := newAppService(gdb)
	const appID, petID, orgID, userID = uint(7), uint(9), uint(3), uint(5)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT * FROM "adoption_applications"`)).
		WithArgs(appID, 1).
		WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "pet_id", "org_id", "status"}).
			AddRow(appID, userID, petID, orgID, constants.AppStatusSubmitted))

	if _, err := svc.UpdateStatus(userID, appID, "user", constants.AppStatusRejected, "x"); err == nil {
		t.Fatal("user must not be able to reject")
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("expectations: %v", err)
	}
}
