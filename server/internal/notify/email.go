package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
	"strconv"
	"strings"
)

type EmailNotifier struct{}

func NewEmailNotifier() *EmailNotifier             { return &EmailNotifier{} }
func (n *EmailNotifier) Type() string              { return "email" }
func (n *EmailNotifier) SensitiveFields() []string { return []string{"password"} }

func (n *EmailNotifier) Validate(config map[string]any) error {
	host := strings.TrimSpace(asString(config["host"]))
	port := asInt(config["port"])
	from := strings.TrimSpace(asString(config["from"]))
	to := strings.TrimSpace(asString(config["to"]))
	if host == "" || port <= 0 || from == "" || to == "" {
		return fmt.Errorf("email host/port/from/to are required")
	}
	return nil
}

func (n *EmailNotifier) Send(_ context.Context, config map[string]any, message Message) error {
	if err := n.Validate(config); err != nil {
		return err
	}
	host := strings.TrimSpace(asString(config["host"]))
	port := asInt(config["port"])
	username := strings.TrimSpace(asString(config["username"]))
	password := strings.TrimSpace(asString(config["password"]))
	from := strings.TrimSpace(asString(config["from"]))
	toList := splitCommaValues(asString(config["to"]))
	address := host + ":" + strconv.Itoa(port)
	headers := []string{"From: " + from, "To: " + strings.Join(toList, ", "), "Subject: " + message.Title, "MIME-Version: 1.0", "Content-Type: text/plain; charset=UTF-8", "", message.Body}
	var auth smtp.Auth
	if username != "" {
		auth = smtp.PlainAuth("", username, password, host)
	}

	rawMessage := []byte(strings.Join(headers, "\r\n"))

	if port == 465 {
		tlsConfig := &tls.Config{ServerName: host}
		conn, err := tls.Dial("tcp", address, tlsConfig)
		if err != nil {
			return fmt.Errorf("dial tls for smtp port 465 failed: %w", err)
		}
		client, err := smtp.NewClient(conn, host)
		if err != nil {
			return fmt.Errorf("create smtp client over tls failed: %w", err)
		}
		defer client.Close()
		if auth != nil {
			if ok, _ := client.Extension("AUTH"); ok {
				if err = client.Auth(auth); err != nil {
					return fmt.Errorf("smtp auth failed: %w", err)
				}
			}
		}
		if err = client.Mail(from); err != nil {
			return fmt.Errorf("smtp mail from failed: %w", err)
		}
		for _, toAddr := range toList {
			if err = client.Rcpt(toAddr); err != nil {
				return fmt.Errorf("smtp rcpt failed for %s: %w", toAddr, err)
			}
		}
		writer, err := client.Data()
		if err != nil {
			return fmt.Errorf("smtp data failed: %w", err)
		}
		if _, err = writer.Write(rawMessage); err != nil {
			return fmt.Errorf("smtp write message failed: %w", err)
		}
		if err = writer.Close(); err != nil {
			return fmt.Errorf("smtp data close failed: %w", err)
		}
		return client.Quit()
	}

	return smtp.SendMail(address, auth, from, toList, rawMessage)
}
