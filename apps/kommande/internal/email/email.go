package email

import (
	"fmt"
	"net/smtp"

	"kommande/internal/config"
	"kommande/internal/logging"
)

func Send(cfg *config.Config, to, subject, body string) {
	if cfg.SMTPHost == "" || to == "" {
		logging.Log.Warn("email skipped", "reason", "smtp or recipient missing", "host_set", cfg.SMTPHost != "", "to_set", to != "")
		return
	}
	go func() {
		addr := cfg.SMTPHost + ":" + cfg.SMTPPort
		msg := []byte(fmt.Sprintf(
			"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
			cfg.SMTPFrom, to, subject, body,
		))
		var auth smtp.Auth
		if cfg.SMTPUser != "" {
			auth = smtp.PlainAuth("", cfg.SMTPUser, cfg.SMTPPassword, cfg.SMTPHost)
		}
		if err := smtp.SendMail(addr, auth, cfg.SMTPFrom, []string{to}, msg); err != nil {
			logging.Log.Error("email delivery failed", "to", to, "subject", subject, "host", cfg.SMTPHost, "err", err)
			return
		}
		logging.Log.Info("email delivered", "to", to, "subject", subject)
	}()
}
