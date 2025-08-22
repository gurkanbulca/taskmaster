// pkg/email/smtp.go
package email

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/smtp"
	"text/template"
	"time"

	ent "github.com/gurkanbulca/taskmaster/ent/generated"
)

// SMTPEmailService implements EmailService using SMTP
type SMTPEmailService struct {
	config    *Config
	templates *Templates
	auth      smtp.Auth
}

// NewSMTPEmailService creates a new SMTP email service
func NewSMTPEmailService(config *Config) *SMTPEmailService {
	auth := smtp.PlainAuth("", config.SMTPUsername, config.SMTPPassword, config.SMTPHost)

	return &SMTPEmailService{
		config:    config,
		templates: NewTemplates(),
		auth:      auth,
	}
}

// SendVerificationEmail sends an email verification email
func (s *SMTPEmailService) SendVerificationEmail(ctx context.Context, user *ent.User, token string) error {
	data := s.buildEmailData(user, token, time.Now().Add(24*time.Hour))
	data.VerificationURL = fmt.Sprintf("%s/verify-email?token=%s", s.config.BaseURL, token)

	return s.sendEmail(ctx, user.Email, s.templates.Verification, data)
}

// SendPasswordResetEmail sends a password reset email
func (s *SMTPEmailService) SendPasswordResetEmail(ctx context.Context, user *ent.User, token string) error {
	data := s.buildEmailData(user, token, time.Now().Add(1*time.Hour))
	data.ResetURL = fmt.Sprintf("%s/reset-password?token=%s", s.config.BaseURL, token)

	return s.sendEmail(ctx, user.Email, s.templates.PasswordReset, data)
}

// SendWelcomeEmail sends a welcome email after email verification
func (s *SMTPEmailService) SendWelcomeEmail(ctx context.Context, user *ent.User) error {
	data := s.buildEmailData(user, "", time.Time{})

	return s.sendEmail(ctx, user.Email, s.templates.Welcome, data)
}

// SendPasswordChangedNotification sends a notification when password is changed
func (s *SMTPEmailService) SendPasswordChangedNotification(ctx context.Context, user *ent.User) error {
	data := s.buildEmailData(user, "", time.Time{})

	return s.sendEmail(ctx, user.Email, s.templates.PasswordChanged, data)
}

// buildEmailData creates EmailData for template rendering
func (s *SMTPEmailService) buildEmailData(user *ent.User, token string, expiresAt time.Time) *EmailData {
	return &EmailData{
		User:         user,
		Token:        token,
		ExpiresAt:    expiresAt,
		SupportEmail: s.config.SupportEmail,
		AppName:      s.config.AppName,
		BaseURL:      s.config.BaseURL,
	}
}

// sendEmail sends an email using SMTP
func (s *SMTPEmailService) sendEmail(ctx context.Context, to string, template EmailTemplate, data *EmailData) error {
	// Render subject
	subjectTmpl, err := s.parseTemplate(template.Subject)
	if err != nil {
		return fmt.Errorf("parse subject template: %w", err)
	}

	var subjectBuf bytes.Buffer
	if err := subjectTmpl.Execute(&subjectBuf, data); err != nil {
		return fmt.Errorf("execute subject template: %w", err)
	}

	// Render HTML body
	htmlTmpl, err := s.parseTemplate(template.HTMLBody)
	if err != nil {
		return fmt.Errorf("parse HTML template: %w", err)
	}

	var htmlBuf bytes.Buffer
	if err := htmlTmpl.Execute(&htmlBuf, data); err != nil {
		return fmt.Errorf("execute HTML template: %w", err)
	}

	// Render text body
	textTmpl, err := s.parseTemplate(template.TextBody)
	if err != nil {
		return fmt.Errorf("parse text template: %w", err)
	}

	var textBuf bytes.Buffer
	if err := textTmpl.Execute(&textBuf, data); err != nil {
		return fmt.Errorf("execute text template: %w", err)
	}

	// Create MIME message
	boundary := s.generateBoundary()
	message := s.buildMIMEMessage(
		s.config.FromEmail,
		s.config.FromName,
		to,
		subjectBuf.String(),
		textBuf.String(),
		htmlBuf.String(),
		boundary,
	)

	// Send email
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)
	err = smtp.SendMail(addr, s.auth, s.config.FromEmail, []string{to}, message)
	if err != nil {
		return fmt.Errorf("send email: %w", err)
	}

	return nil
}

// parseTemplate parses a template string
func (s *SMTPEmailService) parseTemplate(templateStr string) (*template.Template, error) {
	return template.New("email").Parse(templateStr)
}

// generateBoundary generates a random boundary for MIME messages
func (s *SMTPEmailService) generateBoundary() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

// buildMIMEMessage builds a MIME email message with both text and HTML parts
func (s *SMTPEmailService) buildMIMEMessage(from, fromName, to, subject, textBody, htmlBody, boundary string) []byte {
	message := fmt.Sprintf(`From: %s <%s>
To: %s
Subject: %s
MIME-Version: 1.0
Content-Type: multipart/alternative; boundary="%s"

--%s
Content-Type: text/plain; charset=UTF-8
Content-Transfer-Encoding: 7bit

%s

--%s
Content-Type: text/html; charset=UTF-8
Content-Transfer-Encoding: 7bit

%s

--%s--
`, fromName, from, to, subject, boundary, boundary, textBody, boundary, htmlBody, boundary)

	return []byte(message)
}

// TestConnection tests the SMTP connection
func (s *SMTPEmailService) TestConnection(ctx context.Context) error {
	addr := fmt.Sprintf("%s:%d", s.config.SMTPHost, s.config.SMTPPort)

	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial SMTP server: %w", err)
	}
	defer client.Close()

	if err := client.Auth(s.auth); err != nil {
		return fmt.Errorf("SMTP authentication failed: %w", err)
	}

	return nil
}

// MockEmailService implements EmailService for testing
type MockEmailService struct {
	SentEmails []SentEmail
}

// SentEmail represents an email that was sent via MockEmailService
type SentEmail struct {
	To       string
	Template string
	Data     *EmailData
	SentAt   time.Time
}

// NewMockEmailService creates a new mock email service
func NewMockEmailService() *MockEmailService {
	return &MockEmailService{
		SentEmails: make([]SentEmail, 0),
	}
}

// SendVerificationEmail mock implementation
func (m *MockEmailService) SendVerificationEmail(ctx context.Context, user *ent.User, token string) error {
	m.SentEmails = append(m.SentEmails, SentEmail{
		To:       user.Email,
		Template: "verification",
		Data: &EmailData{
			User:  user,
			Token: token,
		},
		SentAt: time.Now(),
	})
	return nil
}

// SendPasswordResetEmail mock implementation
func (m *MockEmailService) SendPasswordResetEmail(ctx context.Context, user *ent.User, token string) error {
	m.SentEmails = append(m.SentEmails, SentEmail{
		To:       user.Email,
		Template: "password_reset",
		Data: &EmailData{
			User:  user,
			Token: token,
		},
		SentAt: time.Now(),
	})
	return nil
}

// SendWelcomeEmail mock implementation
func (m *MockEmailService) SendWelcomeEmail(ctx context.Context, user *ent.User) error {
	m.SentEmails = append(m.SentEmails, SentEmail{
		To:       user.Email,
		Template: "welcome",
		Data: &EmailData{
			User: user,
		},
		SentAt: time.Now(),
	})
	return nil
}

// SendPasswordChangedNotification mock implementation
func (m *MockEmailService) SendPasswordChangedNotification(ctx context.Context, user *ent.User) error {
	m.SentEmails = append(m.SentEmails, SentEmail{
		To:       user.Email,
		Template: "password_changed",
		Data: &EmailData{
			User: user,
		},
		SentAt: time.Now(),
	})
	return nil
}

// GetSentEmails returns all sent emails (for testing)
func (m *MockEmailService) GetSentEmails() []SentEmail {
	return m.SentEmails
}

// GetLastSentEmail returns the last sent email (for testing)
func (m *MockEmailService) GetLastSentEmail() *SentEmail {
	if len(m.SentEmails) == 0 {
		return nil
	}
	return &m.SentEmails[len(m.SentEmails)-1]
}

// Clear clears all sent emails (for testing)
func (m *MockEmailService) Clear() {
	m.SentEmails = make([]SentEmail, 0)
}
