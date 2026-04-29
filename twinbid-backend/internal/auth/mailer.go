package auth

import (
	"fmt"

	"twinbid-backend/internal/config"
	"twinbid-backend/internal/mailer"
)

func sendVerificationEmail(cfg config.SMTPConfig, to, verificationLink string) error {
	body := fmt.Sprintf("Здравствуйте!\n\nДля завершения регистрации в TwinBid подтвердите вашу электронную почту.\n\nПерейдите по ссылке ниже:\n\n%s\n\nЕсли вы не регистрировались в TwinBid, просто проигнорируйте это письмо.\n\nС уважением,\nкоманда TwinBid", verificationLink)
	return mailer.SendEmail(cfg, to, "Подтверждение регистрации TwinBid", body)
}
