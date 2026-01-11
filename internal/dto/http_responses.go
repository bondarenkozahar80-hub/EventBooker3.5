package dto

import (
	"github.com/wb-go/wbf/ginext"
	"time"
)

const (
	FieldBadFormat     = "FIELD_BADFORMAT"
	FieldIncorrect     = "FIELD_INCORRECT"
	ServiceUnavailable = "SERVICE_UNAVAILABLE"
	InternalError      = "Service is currently unavailable. Please try again later."

	EventNotFound         = "EVENT_NOT_FOUND"
	RegistrationNotFound  = "REGISTRATION_NOT_FOUND"
	RegistrationDuplicate = "REGISTRATION_DUPLICATE"
)

type CreateRegistrationRequest struct {
	FullName string `json:"full_name" validate:"required,min=3,max=255"`
	Email    string `json:"email" validate:"required,email"`
	Phone    string `json:"phone" validate:"required"`
}
type RegistrationResponse struct {
	ID        int64     `json:"id"`
	EventID   int64     `json:"event_id"`
	FullName  string    `json:"full_name"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	Status    string    `json:"status"`
	UpdatedAt time.Time `json:"updated_at"`
}
type RegistrationOperateMessage struct {
	RegistrationID int64     `json:"registration_id"`
	EventID        int64     `json:"event_id"`
	ExpireAt       time.Time `json:"expire_at"`
}

type CreateEventRequest struct {
	Name                  string    `json:"name" validate:"required"`
	Description           string    `json:"description"`
	StartTime             time.Time `json:"start_time" validate:"required"`
	EndTime               time.Time `json:"end_time"`
	Location              string    `json:"location"`
	Capacity              int       `json:"capacity" validate:"gt=0"`
	PaymentTimeoutMinutes int       `json:"payment_timeout_minutes" validate:"gte=1"`
}
type ConfirmRegistrationRequest struct {
	EventID int64  `json:"event_id"`
	RegID   int64  `json:"registration_id"`
	Email   string `json:"email"`
	Status  string `json:"status,omitempty"`
}
type EventResponse struct {
	ID                    int64     `json:"id"`
	Name                  string    `json:"name"`
	Description           string    `json:"description"`
	StartTime             time.Time `json:"start_time"`
	EndTime               time.Time `json:"end_time"`
	Location              string    `json:"location"`
	Capacity              int       `json:"capacity"`
	PaymentTimeoutMinutes int       `json:"payment_timeout_minutes"`
	CreatedAt             time.Time `json:"created_at"`
}

type Response struct {
	Status string `json:"status"`
	Error  *Error `json:"error,omitempty"`
	Data   any    `json:"data,omitempty"`
}

type Error struct {
	Code string `json:"code"`
	Desc string `json:"desc"`
}
type EventInfoResponse struct {
	ID                    int64                  `json:"id"`
	Name                  string                 `json:"name"`
	Description           string                 `json:"description"`
	StartTime             time.Time              `json:"start_time"`
	EndTime               time.Time              `json:"end_time"`
	Location              string                 `json:"location"`
	Capacity              int                    `json:"capacity"`
	AvailableSeats        int                    `json:"available_seats"`
	PaymentTimeoutMinutes int                    `json:"payment_timeout_minutes"`
	CreatedAt             time.Time              `json:"created_at"`
	UpdatedAt             time.Time              `json:"updated_at"`
	Registrations         []RegistrationResponse `json:"registrations,omitempty"`
}

func BadResponseError(c *ginext.Context, code, desc string) {
	c.JSON(400, Response{
		Status: "error",
		Error: &Error{
			Code: code,
			Desc: desc,
		},
	})
}

func InternalServerError(c *ginext.Context) {
	c.JSON(500, Response{
		Status: "error",
		Error: &Error{
			Code: ServiceUnavailable,
			Desc: InternalError,
		},
	})
}

func FieldBadFormatError(c *ginext.Context, fieldName string) {
	BadResponseError(c, FieldBadFormat, "Field '"+fieldName+"' has bad format")
}

func FieldIncorrectError(c *ginext.Context, fieldName string) {
	BadResponseError(c, FieldIncorrect, "Field '"+fieldName+"' is incorrect")
}

func EventNotFoundError(c *ginext.Context) {
	BadResponseError(c, EventNotFound, "Event not found")
}

func RegistrationNotFoundError(c *ginext.Context) {
	BadResponseError(c, RegistrationNotFound, "Registration not found")
}

func RegistrationDuplicateError(c *ginext.Context) {
	BadResponseError(c, RegistrationDuplicate, "You have already registered for this event")
}

func SuccessResponse(c *ginext.Context, data any) {
	c.JSON(200, Response{
		Status: "ok",
		Data:   data,
	})
}

func SuccessCreatedResponse(c *ginext.Context, data any) {
	c.JSON(201, Response{
		Status: "ok",
		Data:   data,
	})
}
