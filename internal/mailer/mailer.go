package mailer

import (
	"fmt"
	"net/smtp"

	"github.com/rs/zerolog"
)

func SendRegistrationEmail(log *zerolog.Logger, eventName, status, recipientEmail string, timeout int) error {

	from := "testovyjtestovyj134@gmail.com"
	pass := "kbhc mqxv amed ljxd"

	var subject, body string
	switch status {
	case "confirmed":
		subject = "✅ Ваша регистрация подтверждена"
		body = fmt.Sprintf("Здравствуйте!\n\nВаша регистрация на мероприятие «%s» подтверждена.\nЖдём вас!", eventName)
	case "canceled":
		subject = "❌ Ваша регистрация отменена"
		body = fmt.Sprintf("Здравствуйте!\n\nВаша регистрация на мероприятие «%s» была отменена, так как время подтверждения истекло.", eventName)
	case "pending":
		subject = "❌ Вы начали регистрацию"
		body = fmt.Sprintf("Здравствуйте!\n\nВы начали регистрацию на мероприятие «%s». Необходимо осуществить подтверждение в течение %v минут.\n В ином случае, ваша регистрация будет отменена.", eventName, timeout)

	}

	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\n\r\n%s",
		from, recipientEmail, subject, body,
	)

	smtpServer := "smtp.gmail.com:587"
	auth := smtp.PlainAuth("", from, pass, "smtp.gmail.com")

	if err := smtp.SendMail(smtpServer, auth, from, []string{recipientEmail}, []byte(msg)); err != nil {
		log.Warn().Msgf("Ошибка при отправке email пользователю %s: %v", recipientEmail, err)
		return fmt.Errorf("send email: %w", err)
	}

	log.Info().Msgf("📧 Письмо успешно отправлено пользователю %s (статус: %s)", recipientEmail, status)
	return nil
}
