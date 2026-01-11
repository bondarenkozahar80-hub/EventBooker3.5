package service

import (
	"encoding/json"
	"fifthOne/internal/dto"
	"fifthOne/internal/mailer"
	"fifthOne/internal/model"
	"fifthOne/internal/rabbit"
	"fifthOne/internal/repo"
	"fifthOne/pkg/validator"
	"fmt"
	"github.com/rs/zerolog"
	"github.com/wb-go/wbf/ginext"
	"github.com/wb-go/wbf/zlog"
	"strconv"
	"time"
)

type Service interface {
	CreateEvent(ctx *ginext.Context)
	Book(ctx *ginext.Context)
	Confirm(ctx *ginext.Context)
	GetInfo(ctx *ginext.Context)
	GetAllEvents(ctx *ginext.Context)
}

type service struct {
	repo repo.Repository
	log  *zerolog.Logger
	rbt  *rabbit.Client
}

func NewService(repo repo.Repository, logger *zerolog.Logger, rbt *rabbit.Client) Service {
	return &service{
		repo: repo,
		log:  logger,
		rbt:  rbt,
	}
}

func (s *service) CreateEvent(ctx *ginext.Context) {
	var req dto.CreateEventRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		s.log.Error().Err(err).Msg("failed to parse create event request")
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Invalid JSON format")
		return
	}

	if verr := validator.Validate(ctx, req); verr != nil {
		s.log.Error().Msgf("validation failed: %v", verr)
		dto.BadResponseError(ctx, dto.FieldIncorrect, fmt.Sprintf("%v", verr))
		return
	}

	event := &model.Event{
		Name:                  req.Name,
		Description:           req.Description,
		StartTime:             req.StartTime,
		EndTime:               req.EndTime,
		Location:              req.Location,
		Capacity:              req.Capacity,
		PaymentTimeoutMinutes: req.PaymentTimeoutMinutes,
		CreatedAt:             time.Now(),
		UpdatedAt:             time.Now(),
	}

	id, err := s.repo.CreateEvent(ctx, event)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to create event in DB")
		dto.InternalServerError(ctx)
		return
	}

	event.ID = int(id)
	s.log.Info().Int64("event_id", id).Msg("event created successfully")

	dto.SuccessCreatedResponse(ctx, dto.EventResponse{
		ID:                    int64(event.ID),
		Name:                  event.Name,
		Description:           event.Description,
		StartTime:             event.StartTime,
		EndTime:               event.EndTime,
		Location:              event.Location,
		Capacity:              event.Capacity,
		PaymentTimeoutMinutes: event.PaymentTimeoutMinutes,
		CreatedAt:             event.CreatedAt,
	})
}

func (s *service) Book(ctx *ginext.Context) {
	eventID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Invalid event ID")
		return
	}

	var req dto.CreateRegistrationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Invalid JSON format")
		return
	}

	if verr := validator.Validate(ctx, req); verr != nil {
		dto.BadResponseError(ctx, dto.FieldIncorrect, fmt.Sprintf("%v", verr))
		return
	}

	registration := &model.Registration{
		EventID:  int(eventID),
		FullName: req.FullName,
		Email:    req.Email,
		Phone:    req.Phone,
		Status:   "pending",
	}

	id, timeout, err := s.repo.BookRegistrationTx(ctx.Request.Context(), registration)
	if err != nil {
		switch err {
		case repo.ErrEventNotFound:
			dto.EventNotFoundError(ctx)
			return
		case repo.ErrEventFull:
			dto.BadResponseError(ctx, dto.FieldIncorrect, "Event is full")
			return
		case repo.ErrDuplicateRegistration:
			dto.RegistrationDuplicateError(ctx)
			return
		default:
			s.log.Error().Err(err).Msg("failed to book registration")
			dto.InternalServerError(ctx)
			return
		}
	}

	s.log.Info().Int64("registration_id", id).Msg("registration created successfully")

	msg := dto.RegistrationOperateMessage{
		RegistrationID: id,
		EventID:        eventID,
		ExpireAt:       time.Now().Add(time.Duration(timeout) * time.Minute),
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to marshal cancel message")
		dto.InternalServerError(ctx)
		return
	}
	delaySeconds := timeout * 60
	if err := s.rbt.Publish(payload, delaySeconds); err != nil {
		s.log.Error().Err(err).Msg("failed to publish cancel message to RabbitMQ")
	}

	event, err := s.repo.GetEventByID(ctx, int64(registration.EventID))
	if err != nil {
		zlog.Logger.Error().Err(err).Msg("Failed to get event from DB in worker")
	}

	if err := mailer.SendRegistrationEmail(
		&zlog.Logger,
		event.Name,
		"pending",
		registration.Email,
		event.PaymentTimeoutMinutes,
	); err != nil {
		zlog.Logger.Warn().Err(err).Msg("Failed to send notification on e-mail")
	}

	dto.SuccessCreatedResponse(ctx, dto.RegistrationResponse{
		ID:        id,
		EventID:   eventID,
		FullName:  req.FullName,
		Email:     req.Email,
		CreatedAt: time.Now(),
		Status:    registration.Status,
	})
}

func (s *service) Confirm(ctx *ginext.Context) {
	eventID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Invalid event ID")
		return
	}

	var req dto.ConfirmRegistrationRequest
	if err := ctx.ShouldBindJSON(&req); err != nil {
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Invalid JSON format")
		return
	}

	if verr := validator.Validate(ctx, req); verr != nil {
		dto.BadResponseError(ctx, dto.FieldIncorrect, fmt.Sprintf("%v", verr))
		return
	}

	event, err := s.repo.GetEventByID(ctx, eventID)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to get event for confirm")
		dto.EventNotFoundError(ctx)
		return
	}

	reg, err := s.repo.GetRegistrationByRegID(ctx, req.RegID)
	if err != nil {
		s.log.Error().Err(err).Msg("registration not found for confirmation")
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Registration not found")
		return
	}
	if reg.Email != req.Email {
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Wrong email for this registration")
		return
	}

	if reg.Status == "confirmed" {
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Already confirmed")
		return
	}

	if reg.Status == "canceled" {
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Registration was canceled")
		return
	}

	if err := s.repo.UpdateRegistrationStatusTx(ctx, int64(reg.ID), "confirmed"); err != nil {
		s.log.Error().Err(err).Msg("failed to update registration status to confirmed")
		dto.InternalServerError(ctx)
		return
	}

	s.log.Info().
		Int("registration_id", reg.ID).
		Str("email", reg.Email).
		Msg("registration confirmed successfully")

	if err := mailer.SendRegistrationEmail(s.log, event.Name, "confirmed", reg.Email, 0); err != nil {
		zlog.Logger.Warn().Err(err).Msg("Failed to send successful registration notification on e-mail")
	}
	dto.SuccessResponse(ctx, dto.RegistrationResponse{
		ID:        int64(reg.ID),
		EventID:   eventID,
		FullName:  reg.FullName,
		Email:     reg.Email,
		Status:    "confirmed",
		UpdatedAt: time.Now(),
	})
}

func (s *service) GetInfo(ctx *ginext.Context) {
	eventID, err := strconv.ParseInt(ctx.Param("id"), 10, 64)
	if err != nil {
		dto.BadResponseError(ctx, dto.FieldIncorrect, "Invalid event ID")
		return
	}

	isAdmin := ctx.Query("admin") == "true"

	event, err := s.repo.GetEventByID(ctx, eventID)
	if err != nil {
		dto.EventNotFoundError(ctx)
		return
	}

	count, err := s.repo.CountRegistrations(ctx, eventID)
	if err != nil {
		s.log.Error().Err(err).Msg("failed to count registrations")
		dto.InternalServerError(ctx)
		return
	}

	resp := dto.EventInfoResponse{
		ID:                    int64(event.ID),
		Name:                  event.Name,
		Description:           event.Description,
		StartTime:             event.StartTime,
		EndTime:               event.EndTime,
		Location:              event.Location,
		Capacity:              event.Capacity,
		PaymentTimeoutMinutes: event.PaymentTimeoutMinutes,
		CreatedAt:             event.CreatedAt,
		UpdatedAt:             event.UpdatedAt,
		AvailableSeats:        event.Capacity - count,
	}

	if isAdmin {
		registrations, err := s.repo.GetRegistrationsByEventID(ctx, eventID)
		if err != nil {
			s.log.Error().Err(err).Msg("failed to get registrations for admin view")
			dto.InternalServerError(ctx)
			return
		}

		for _, r := range registrations {
			resp.Registrations = append(resp.Registrations, dto.RegistrationResponse{
				ID:        int64(r.ID),
				EventID:   int64(r.EventID),
				FullName:  r.FullName,
				Email:     r.Email,
				Status:    r.Status,
				CreatedAt: r.CreatedAt,
				UpdatedAt: r.UpdatedAt,
			})
		}
	}

	dto.SuccessResponse(ctx, resp)
}

func (s *service) GetAllEvents(ctx *ginext.Context) {
	isAdmin := ctx.Query("admin") == "true"

	events, err := s.repo.GetAllEvents(ctx)
	if err != nil {
		dto.InternalServerError(ctx)
		return
	}

	resp := make([]dto.EventInfoResponse, 0, len(events))

	for _, e := range events {
		count, err := s.repo.CountRegistrations(ctx, int64(e.ID))
		if err != nil {
			s.log.Error().Err(err).Msg("failed to count registrations for event")
			continue
		}

		item := dto.EventInfoResponse{
			ID:                    int64(e.ID),
			Name:                  e.Name,
			Description:           e.Description,
			StartTime:             e.StartTime,
			EndTime:               e.EndTime,
			Location:              e.Location,
			Capacity:              e.Capacity,
			AvailableSeats:        e.Capacity - count,
			PaymentTimeoutMinutes: e.PaymentTimeoutMinutes,
			CreatedAt:             e.CreatedAt,
			UpdatedAt:             e.UpdatedAt,
		}

		if isAdmin {
			registrations, _ := s.repo.GetRegistrationsByEventID(ctx, int64(e.ID))
			for _, r := range registrations {
				item.Registrations = append(item.Registrations, dto.RegistrationResponse{
					ID:        int64(r.ID),
					EventID:   int64(r.EventID),
					FullName:  r.FullName,
					Email:     r.Email,
					Status:    r.Status,
					CreatedAt: r.CreatedAt,
					UpdatedAt: r.UpdatedAt,
				})
			}
		}

		resp = append(resp, item)
	}

	dto.SuccessResponse(ctx, resp)
}
