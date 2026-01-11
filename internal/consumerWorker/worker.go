package consumerWorker

import (
	"context"
	"encoding/json"
	"fifthOne/internal/dto"
	"fifthOne/internal/mailer"

	"fifthOne/internal/rabbit"
	"fifthOne/internal/repo"

	"github.com/wb-go/wbf/zlog"
)

type Reader struct {
	RMQ    *rabbit.Client
	repo   repo.Repository
	done   chan struct{}
	cancel context.CancelFunc
}

func NewReader(rmq *rabbit.Client, repo repo.Repository) *Reader {
	return &Reader{
		RMQ:  rmq,
		repo: repo,
		done: make(chan struct{}),
	}
}

func (r *Reader) Start(ctx context.Context) {
	cctx, cancel := context.WithCancel(ctx)
	r.cancel = cancel

	zlog.Logger.Info().Msg("🐇 RabbitMQ Reader started")

	go func() {
		defer close(r.done)

		handler := func(body []byte) error {
			var msg dto.RegistrationOperateMessage
			if err := json.Unmarshal(body, &msg); err != nil {
				zlog.Logger.Error().
					Err(err).
					Msgf("Failed to unmarshal message: %s", string(body))
				return err
			}

			zlog.Logger.Info().
				Int64("registration_id", msg.RegistrationID).
				Int64("event_id", msg.EventID).
				Msg("📩 Received message from RabbitMQ")

			canceled, err := r.repo.CancelIfNotConfirmedTx(cctx, msg.RegistrationID)
			if err != nil {
				zlog.Logger.Error().
					Err(err).
					Int64("registration_id", msg.RegistrationID).
					Msg("Failed to cancel registration (DB operation)")
				return err
			}

			if !canceled {
				zlog.Logger.Info().
					Int64("registration_id", msg.RegistrationID).
					Msg("⏳ Registration already confirmed or canceled — skipping email")
				return nil
			}

			reg, err := r.repo.GetRegistrationByID(ctx, msg.RegistrationID)
			if err != nil {
				zlog.Logger.Error().
					Err(err).
					Int64("registration_id", msg.RegistrationID).
					Msg("Failed to get registration from DB in worker")
				return nil
			}

			event, err := r.repo.GetEventByID(ctx, int64(reg.EventID))
			if err != nil {
				zlog.Logger.Error().
					Err(err).
					Int64("event_id", int64(reg.EventID)).
					Msg("Failed to get event from DB in worker")
				return nil
			}

			if err := mailer.SendRegistrationEmail(
				&zlog.Logger,
				event.Name,
				"canceled",
				reg.Email,
				0,
			); err != nil {
				zlog.Logger.Warn().
					Err(err).
					Msg("Failed to send notification on e-mail")
			} else {
				zlog.Logger.Info().
					Str("email", reg.Email).
					Int64("registration_id", msg.RegistrationID).
					Msg("📧 Cancellation email sent successfully")
			}

			return nil
		}

		if err := r.RMQ.Consume(handler); err != nil {
			zlog.Logger.Error().Err(err).Msg("Failed to start consuming")
			return
		}

		<-cctx.Done()
		zlog.Logger.Info().Msg("🛑 RabbitMQ Reader stopped by context")
	}()
}

func (r *Reader) Stop() {
	if r.cancel != nil {
		r.cancel()
		<-r.done
	}
}
