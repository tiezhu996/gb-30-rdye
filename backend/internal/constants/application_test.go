package constants

import "testing"

func TestApplicationStatusTerminalAndActive(t *testing.T) {
	terminal := []string{AppStatusApproved, AppStatusRejected, AppStatusWithdrawn}
	for _, s := range terminal {
		if !IsTerminalApplicationStatus(s) {
			t.Errorf("IsTerminalApplicationStatus(%s) = false, want true", s)
		}
		if IsActiveApplicationStatus(s) {
			t.Errorf("IsActiveApplicationStatus(%s) = true, want false", s)
		}
	}
	active := []string{AppStatusSubmitted, AppStatusOrgReview, AppStatusCommunicating, AppStatusConfirmed, AppStatusOfflineInterview}
	for _, s := range active {
		if IsTerminalApplicationStatus(s) {
			t.Errorf("IsTerminalApplicationStatus(%s) = true, want false", s)
		}
		if !IsActiveApplicationStatus(s) {
			t.Errorf("IsActiveApplicationStatus(%s) = false, want true", s)
		}
	}
	if !IsValidApplicationStatus(AppStatusWithdrawn) {
		t.Error("withdrawn must be a valid status")
	}
}

func TestCanOrgTransition(t *testing.T) {
	// Pipeline forwards and reject from every active stage.
	allowed := [][2]string{
		{AppStatusSubmitted, AppStatusOrgReview},
		{AppStatusSubmitted, AppStatusRejected},
		{AppStatusOrgReview, AppStatusCommunicating},
		{AppStatusOrgReview, AppStatusRejected},
		{AppStatusCommunicating, AppStatusConfirmed},
		{AppStatusCommunicating, AppStatusRejected},
		{AppStatusConfirmed, AppStatusOfflineInterview},
		{AppStatusConfirmed, AppStatusRejected},
		{AppStatusOfflineInterview, AppStatusApproved},
		{AppStatusOfflineInterview, AppStatusRejected},
	}
	for _, c := range allowed {
		if !CanOrgTransition(c[0], c[1]) {
			t.Errorf("CanOrgTransition(%s -> %s) = false, want true", c[0], c[1])
		}
	}
	// No skipping stages, no withdrawing, no touching terminal records.
	denied := [][2]string{
		{AppStatusSubmitted, AppStatusApproved},
		{AppStatusSubmitted, AppStatusWithdrawn},
		{AppStatusOrgReview, AppStatusApproved},
		{AppStatusApproved, AppStatusRejected},
		{AppStatusRejected, AppStatusSubmitted},
		{AppStatusWithdrawn, AppStatusApproved},
	}
	for _, c := range denied {
		if CanOrgTransition(c[0], c[1]) {
			t.Errorf("CanOrgTransition(%s -> %s) = true, want false", c[0], c[1])
		}
	}
}

func TestCanUserWithdraw(t *testing.T) {
	for _, s := range ActiveApplicationStatuses() {
		if !CanUserWithdraw(s) {
			t.Errorf("CanUserWithdraw(%s) = false, want true", s)
		}
	}
	for _, s := range []string{AppStatusApproved, AppStatusRejected, AppStatusWithdrawn} {
		if CanUserWithdraw(s) {
			t.Errorf("CanUserWithdraw(%s) = true, want false", s)
		}
	}
}
