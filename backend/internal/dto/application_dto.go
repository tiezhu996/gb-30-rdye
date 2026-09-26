package dto

// ApplicationSubmitRequest submits an adoption application.
type ApplicationSubmitRequest struct {
	PetID         uint   `json:"pet_id" binding:"required"`
	Questionnaire string `json:"questionnaire"`
}

// ApplicationStatusRequest changes application status.
// Reason records why the application ended and is kept on the record
// for rejected/withdrawn applications.
type ApplicationStatusRequest struct {
	Status string `json:"status" binding:"required"`
	Reason string `json:"reason"`
}
