package constants

// ApplicationStatus enumerates the adoption application state machine.
const (
	AppStatusSubmitted        = "submitted"
	AppStatusOrgReview        = "org_review"
	AppStatusCommunicating    = "communicating"
	AppStatusConfirmed        = "confirmed"
	AppStatusOfflineInterview = "offline_interview"
	AppStatusApproved         = "approved"
	AppStatusRejected         = "rejected"
	AppStatusWithdrawn        = "withdrawn"
)

// ValidApplicationStatuses returns all accepted application statuses.
func ValidApplicationStatuses() []string {
	return []string{
		AppStatusSubmitted, AppStatusOrgReview, AppStatusCommunicating,
		AppStatusConfirmed, AppStatusOfflineInterview, AppStatusApproved,
		AppStatusRejected, AppStatusWithdrawn,
	}
}

// IsValidApplicationStatus reports whether a status is known.
func IsValidApplicationStatus(s string) bool {
	for _, v := range ValidApplicationStatuses() {
		if v == s {
			return true
		}
	}
	return false
}

// IsTerminalApplicationStatus reports whether an application is closed and must never change again.
// approved（已领养）、rejected（机构拒绝）、withdrawn（申请人撤回）都是终态。
func IsTerminalApplicationStatus(s string) bool {
	return s == AppStatusApproved || s == AppStatusRejected || s == AppStatusWithdrawn
}

// ActiveApplicationStatuses returns statuses that still hold the pet as "pending".
// An application in one of these statuses blocks other users from applying for the same pet.
func ActiveApplicationStatuses() []string {
	return []string{
		AppStatusSubmitted, AppStatusOrgReview, AppStatusCommunicating,
		AppStatusConfirmed, AppStatusOfflineInterview,
	}
}

// IsActiveApplicationStatus reports whether an application is still in progress.
func IsActiveApplicationStatus(s string) bool {
	for _, v := range ActiveApplicationStatuses() {
		if v == s {
			return true
		}
	}
	return false
}

// NextApplicationStatuses returns the allowed forward transitions driven by the org.
// rejected is reachable from every active status; approved/offline_interview/... keep the review pipeline.
func NextApplicationStatuses(s string) []string {
	switch s {
	case AppStatusSubmitted:
		return []string{AppStatusOrgReview, AppStatusRejected}
	case AppStatusOrgReview:
		return []string{AppStatusCommunicating, AppStatusRejected}
	case AppStatusCommunicating:
		return []string{AppStatusConfirmed, AppStatusRejected}
	case AppStatusConfirmed:
		return []string{AppStatusOfflineInterview, AppStatusRejected}
	case AppStatusOfflineInterview:
		return []string{AppStatusApproved, AppStatusRejected}
	default:
		return nil
	}
}

// CanOrgTransition reports whether an org may move an application from -> to.
// Orgs drive the review pipeline forward and may reject at any active stage.
// They can neither approve out of the pipeline order nor touch terminal applications.
func CanOrgTransition(from, to string) bool {
	if !IsActiveApplicationStatus(from) {
		return false
	}
	for _, s2 := range NextApplicationStatuses(from) {
		if s2 == to {
			return true
		}
	}
	return false
}

// CanUserWithdraw reports whether the applicant may withdraw an application in the given status.
// Withdrawal is only allowed while the application is active; an approved application is immutable.
func CanUserWithdraw(s string) bool {
	return IsActiveApplicationStatus(s)
}
