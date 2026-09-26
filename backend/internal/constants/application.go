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

// IsTerminalApplicationStatus reports whether the application is closed.
// Terminal applications cannot be transitioned any further.
func IsTerminalApplicationStatus(s string) bool {
	switch s {
	case AppStatusApproved, AppStatusRejected, AppStatusWithdrawn:
		return true
	}
	return false
}

// IsActiveApplicationStatus reports whether the application is still open
// (i.e. the pet is locked and not available for new applications).
func IsActiveApplicationStatus(s string) bool {
	return IsValidApplicationStatus(s) && !IsTerminalApplicationStatus(s)
}

// NextApplicationStatuses returns the allowed forward transitions.
func NextApplicationStatuses(s string) []string {
	switch s {
	case AppStatusSubmitted:
		return []string{AppStatusOrgReview, AppStatusRejected, AppStatusWithdrawn}
	case AppStatusOrgReview:
		return []string{AppStatusCommunicating, AppStatusRejected, AppStatusWithdrawn}
	case AppStatusCommunicating:
		return []string{AppStatusConfirmed, AppStatusRejected, AppStatusWithdrawn}
	case AppStatusConfirmed:
		return []string{AppStatusOfflineInterview, AppStatusRejected, AppStatusWithdrawn}
	case AppStatusOfflineInterview:
		return []string{AppStatusApproved, AppStatusRejected, AppStatusWithdrawn}
	default:
		return nil
	}
}
