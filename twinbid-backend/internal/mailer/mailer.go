package mailer

import (
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strings"

	"twinbid-backend/internal/config"
)

func SendEmail(cfg config.SMTPConfig, to, subject, body string) error {
	msg := []byte("To: " + to + "\r\n" +
		"From: " + cfg.From + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"MIME-Version: 1.0\r\n" +
		"Content-Type: text/plain; charset=UTF-8\r\n\r\n" + body)

	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	tlsCfg := &tls.Config{ServerName: cfg.Host}
	var (
		client *smtp.Client
		conn   net.Conn
		err    error
	)
	if strings.EqualFold(cfg.TLSType, "starttls") {
		conn, err = net.Dial("tcp", addr)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			conn.Close()
			return err
		}
		if err := client.StartTLS(tlsCfg); err != nil {
			client.Close()
			conn.Close()
			return err
		}
	} else {
		conn, err = tls.Dial("tcp", addr, tlsCfg)
		if err != nil {
			return err
		}
		client, err = smtp.NewClient(conn, cfg.Host)
		if err != nil {
			conn.Close()
			return err
		}
	}
	defer conn.Close()
	defer client.Quit()

	if err := client.Auth(auth); err != nil {
		return err
	}
	if err := client.Mail(cfg.From); err != nil {
		return err
	}
	if err := client.Rcpt(to); err != nil {
		return err
	}
	w, err := client.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(msg); err != nil {
		return err
	}
	return w.Close()
}
