package dto

// ApplicationSubmitRequest submits an adoption application.
type ApplicationSubmitRequest struct {
	PetID         uint   `json:"pet_id" binding:"required"`
	Questionnaire string `json:"questionnaire"`
}

// ApplicationStatusRequest changes application status.
// Reason is required when an org rejects or an applicant withdraws, and is
// persisted as the close reason shown in the application history.
type ApplicationStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}
