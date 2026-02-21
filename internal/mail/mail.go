package mail

import (
	"fmt"
	"net/smtp"

	"github.com/taskflow/backend/internal/config"
)

type EmailService struct {
	config config.SMTPConfig
}

func NewEmailService(cfg config.SMTPConfig) *EmailService {
	return &EmailService{config: cfg}
}

func (s *EmailService) SendEmail(to, subject, body string) error {
	if s.config.Host == "" {
		fmt.Printf("\n--- EMAIL WOULD BE SENT ---\nTo: %s\nSubject: %s\nBody: %s\n---------------------------\n", to, subject, body)
		return nil
	}

	auth := smtp.PlainAuth("", s.config.User, s.config.Password, s.config.Host)

	header := make(map[string]string)
	header["From"] = s.config.From
	header["To"] = to
	header["Subject"] = subject
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/html; charset=\"utf-8\""

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + body

	addr := fmt.Sprintf("%s:%s", s.config.Host, s.config.Port)

	// Send email
	err := smtp.SendMail(addr, auth, s.config.From, []string{to}, []byte(message))
	if err != nil {
		return fmt.Errorf("failed to send email: %w", err)
	}

	return nil
}

func (s *EmailService) SendPasswordResetEmail(to, resetURL string) error {
	subject := "Password Reset Request"
	body := fmt.Sprintf(`
		<h1>Password Reset</h1>
		<p>You requested a password reset for your TaskFlow account.</p>
		<p>Click the link below to reset your password:</p>
		<p><a href="%s">%s</a></p>
		<p>If you didn't request this, you can safely ignore this email.</p>
		<p>This link will expire in 1 hour.</p>
	`, resetURL, resetURL)

	return s.SendEmail(to, subject, body)
}

// SendTempPasswordEmail sends login credentials to a newly admin-created user.
func (s *EmailService) SendTempPasswordEmail(to, name, tempPassword string) error {
	subject := "Your TaskFlow Account Has Been Created"
	displayName := name
	if displayName == "" {
		displayName = to
	}
	body := fmt.Sprintf(`
		<h1>Welcome to TaskFlow!</h1>
		<p>Hi %s,</p>
		<p>An administrator has created an account for you on TaskFlow.</p>
		<p><strong>Your login credentials:</strong></p>
		<ul>
			<li><strong>Email:</strong> %s</li>
			<li><strong>Temporary Password:</strong> <code>%s</code></li>
		</ul>
		<p>Please log in and change your password immediately for security reasons.</p>
		<p>If you did not expect this email, please contact your administrator.</p>
	`, displayName, to, tempPassword)

	return s.SendEmail(to, subject, body)
}
