// pkg/email/service.go
package email

import (
	"context"
	"time"

	ent "github.com/gurkanbulca/taskmaster/ent/generated"
)

// EmailService defines the interface for sending emails
type EmailService interface {
	SendVerificationEmail(ctx context.Context, user *ent.User, token string) error
	SendPasswordResetEmail(ctx context.Context, user *ent.User, token string) error
	SendWelcomeEmail(ctx context.Context, user *ent.User) error
	SendPasswordChangedNotification(ctx context.Context, user *ent.User) error
}

// EmailTemplate represents an email template
type EmailTemplate struct {
	Subject  string
	HTMLBody string
	TextBody string
}

// EmailData contains data for template rendering
type EmailData struct {
	User            *ent.User
	Token           string
	ExpiresAt       time.Time
	SupportEmail    string
	AppName         string
	BaseURL         string
	VerificationURL string
	ResetURL        string
}

// Config holds email service configuration
type Config struct {
	SMTPHost     string
	SMTPPort     int
	SMTPUsername string
	SMTPPassword string
	FromEmail    string
	FromName     string
	BaseURL      string
	AppName      string
	SupportEmail string
}

// Templates holds all email templates
type Templates struct {
	Verification    EmailTemplate
	PasswordReset   EmailTemplate
	Welcome         EmailTemplate
	PasswordChanged EmailTemplate
	AccountLocked   EmailTemplate
	SecurityAlert   EmailTemplate
}

// NewTemplates creates default email templates
func NewTemplates() *Templates {
	return &Templates{
		Verification: EmailTemplate{
			Subject: "Verify your {{.AppName}} account",
			HTMLBody: `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Email Verification</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; margin-bottom: 30px; }
        .button { display: inline-block; padding: 12px 24px; background-color: #007bff; color: white; text-decoration: none; border-radius: 5px; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 14px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Welcome to {{.AppName}}!</h1>
        </div>
        
        <p>Hi {{.User.FirstName}},</p>
        
        <p>Thank you for signing up for {{.AppName}}. To complete your registration, please verify your email address by clicking the button below:</p>
        
        <p style="text-align: center; margin: 30px 0;">
            <a href="{{.VerificationURL}}" class="button">Verify Email Address</a>
        </p>
        
        <p>If the button doesn't work, you can copy and paste this link into your browser:</p>
        <p><a href="{{.VerificationURL}}">{{.VerificationURL}}</a></p>
        
        <p>This verification link will expire on {{.ExpiresAt.Format "January 2, 2006 at 3:04 PM"}}.</p>
        
        <p>If you didn't create an account with {{.AppName}}, you can safely ignore this email.</p>
        
        <div class="footer">
            <p>Best regards,<br>The {{.AppName}} Team</p>
            <p>If you have any questions, please contact us at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a></p>
        </div>
    </div>
</body>
</html>`,
			TextBody: `Welcome to {{.AppName}}!

Hi {{.User.FirstName}},

Thank you for signing up for {{.AppName}}. To complete your registration, please verify your email address by visiting this link:

{{.VerificationURL}}

This verification link will expire on {{.ExpiresAt.Format "January 2, 2006 at 3:04 PM"}}.

If you didn't create an account with {{.AppName}}, you can safely ignore this email.

Best regards,
The {{.AppName}} Team

If you have any questions, please contact us at {{.SupportEmail}}`,
		},

		PasswordReset: EmailTemplate{
			Subject: "Reset your {{.AppName}} password",
			HTMLBody: `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Password Reset</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; margin-bottom: 30px; }
        .button { display: inline-block; padding: 12px 24px; background-color: #dc3545; color: white; text-decoration: none; border-radius: 5px; }
        .alert { background-color: #fff3cd; border: 1px solid #ffeaa7; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 14px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Reset Request</h1>
        </div>
        
        <p>Hi {{.User.FirstName}},</p>
        
        <p>We received a request to reset your password for your {{.AppName}} account.</p>
        
        <p style="text-align: center; margin: 30px 0;">
            <a href="{{.ResetURL}}" class="button">Reset Password</a>
        </p>
        
        <p>If the button doesn't work, you can copy and paste this link into your browser:</p>
        <p><a href="{{.ResetURL}}">{{.ResetURL}}</a></p>
        
        <div class="alert">
            <strong>Important:</strong> This password reset link will expire on {{.ExpiresAt.Format "January 2, 2006 at 3:04 PM"}}. For security reasons, you'll need to request a new reset link after this time.
        </div>
        
        <p>If you didn't request a password reset, please ignore this email. Your password will remain unchanged.</p>
        
        <div class="footer">
            <p>Best regards,<br>The {{.AppName}} Team</p>
            <p>If you have any questions, please contact us at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a></p>
        </div>
    </div>
</body>
</html>`,
			TextBody: `Password Reset Request

Hi {{.User.FirstName}},

We received a request to reset your password for your {{.AppName}} account.

Please click the following link to reset your password:
{{.ResetURL}}

This password reset link will expire on {{.ExpiresAt.Format "January 2, 2006 at 3:04 PM"}}.

If you didn't request a password reset, please ignore this email. Your password will remain unchanged.

Best regards,
The {{.AppName}} Team

If you have any questions, please contact us at {{.SupportEmail}}`,
		},

		Welcome: EmailTemplate{
			Subject: "Welcome to {{.AppName}} - Get Started!",
			HTMLBody: `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Welcome to {{.AppName}}</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; margin-bottom: 30px; }
        .button { display: inline-block; padding: 12px 24px; background-color: #28a745; color: white; text-decoration: none; border-radius: 5px; }
        .feature { margin: 20px 0; padding: 15px; background-color: #f8f9fa; border-radius: 5px; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 14px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🎉 Welcome to {{.AppName}}!</h1>
        </div>
        
        <p>Hi {{.User.FirstName}},</p>
        
        <p>Your email has been verified and your {{.AppName}} account is now ready to use! We're excited to have you on board.</p>
        
        <div class="feature">
            <h3>🚀 Getting Started</h3>
            <p>Here are some things you can do to get the most out of {{.AppName}}:</p>
            <ul>
                <li>Create your first task and start organizing your work</li>
                <li>Set up your profile with your preferences</li>
                <li>Explore the task management features</li>
            </ul>
        </div>
        
        <p style="text-align: center; margin: 30px 0;">
            <a href="{{.BaseURL}}" class="button">Start Using {{.AppName}}</a>
        </p>
        
        <div class="footer">
            <p>Best regards,<br>The {{.AppName}} Team</p>
            <p>Need help? Contact us at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a></p>
        </div>
    </div>
</body>
</html>`,
			TextBody: `Welcome to {{.AppName}}!

Hi {{.User.FirstName}},

Your email has been verified and your {{.AppName}} account is now ready to use! We're excited to have you on board.

Getting Started:
- Create your first task and start organizing your work
- Set up your profile with your preferences
- Explore the task management features

Start using {{.AppName}}: {{.BaseURL}}

Best regards,
The {{.AppName}} Team

Need help? Contact us at {{.SupportEmail}}`,
		},

		PasswordChanged: EmailTemplate{
			Subject: "Your {{.AppName}} password has been changed",
			HTMLBody: `
<!DOCTYPE html>
<html>
<head>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Password Changed</title>
    <style>
        body { font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif; line-height: 1.6; color: #333; }
        .container { max-width: 600px; margin: 0 auto; padding: 20px; }
        .header { text-align: center; margin-bottom: 30px; }
        .alert { background-color: #d1ecf1; border: 1px solid #bee5eb; padding: 15px; border-radius: 5px; margin: 20px 0; }
        .footer { margin-top: 30px; padding-top: 20px; border-top: 1px solid #eee; font-size: 14px; color: #666; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>Password Changed Successfully</h1>
        </div>
        
        <p>Hi {{.User.FirstName}},</p>
        
        <p>This is to confirm that your {{.AppName}} account password has been successfully changed.</p>
        
        <div class="alert">
            <strong>Security Notice:</strong> If you didn't make this change, please contact our support team immediately at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a>.
        </div>
        
        <p>For your security, we recommend:</p>
        <ul>
            <li>Using a strong, unique password</li>
            <li>Not sharing your password with anyone</li>
            <li>Logging out of {{.AppName}} on shared devices</li>
        </ul>
        
        <div class="footer">
            <p>Best regards,<br>The {{.AppName}} Team</p>
            <p>If you have any questions, please contact us at <a href="mailto:{{.SupportEmail}}">{{.SupportEmail}}</a></p>
        </div>
    </div>
</body>
</html>`,
			TextBody: `Password Changed Successfully

Hi {{.User.FirstName}},

This is to confirm that your {{.AppName}} account password has been successfully changed.

Security Notice: If you didn't make this change, please contact our support team immediately at {{.SupportEmail}}.

For your security, we recommend:
- Using a strong, unique password
- Not sharing your password with anyone
- Logging out of {{.AppName}} on shared devices

Best regards,
The {{.AppName}} Team

If you have any questions, please contact us at {{.SupportEmail}}`,
		},
	}
}
