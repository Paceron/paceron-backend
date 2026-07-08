package user

type StatusChangeRequest struct {
	Status string `json:"status" binding:"required"`
}
