package mail

import (
	"fmt"
	"log"
	"net/smtp"
	"os"
)

// Send mengirim email teks via SMTP. Tanpa kredensial → tulis ke log (seperti MAIL_MAILER=log).
func Send(to, subject, body string) error {
	host, port, user, pass, from := os.Getenv("MAIL_HOST"), os.Getenv("MAIL_PORT"), os.Getenv("MAIL_USERNAME"), os.Getenv("MAIL_PASSWORD"), os.Getenv("MAIL_FROM")
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n%s", from, to, subject, body)
	if user == "" {
		log.Printf("[mail:log]\n%s\n", msg)
		return nil
	}
	return smtp.SendMail(host+":"+port, smtp.PlainAuth("", user, pass, host), from, []string{to}, []byte(msg))
}
