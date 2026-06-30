package logic

import (
	"fmt"
	"strings"
)

// emailVerificationHTML builds the HTML body for the email verification message.
func emailVerificationHTML(name, verificationURL string) string {
	if strings.TrimSpace(name) == "" {
		name = "there"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Verify your email</title></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;background:#f8fafc;margin:0;padding:0;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;padding:32px;">
        <tr><td>
          <h1 style="margin:0 0 8px;font-size:20px;color:#0f172a;">Verify your email</h1>
          <p style="margin:0 0 20px;font-size:15px;color:#475569;line-height:1.5;">Hi %s,</p>
          <p style="margin:0 0 24px;font-size:15px;color:#475569;line-height:1.5;">
            Welcome! Please confirm your email address to activate your account. This link expires in 1 hour.
          </p>
          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#0d9488;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;padding:12px 24px;border-radius:8px;">Verify email</a>
          </p>
          <p style="margin:0;font-size:13px;color:#94a3b8;line-height:1.5;">
            If you didn&apos;t create an account, you can safely ignore this email.<br>
            Or copy this link: <span style="word-break:break-all;">%s</span>
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, name, verificationURL, verificationURL)
}

// passwordResetHTML builds the HTML body for the password reset message.
func passwordResetHTML(name, resetURL string) string {
	if strings.TrimSpace(name) == "" {
		name = "there"
	}
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><meta charset="utf-8"><title>Reset your password</title></head>
<body style="font-family:-apple-system,BlinkMacSystemFont,Segoe UI,Roboto,sans-serif;background:#f8fafc;margin:0;padding:0;">
  <table width="100%%" cellpadding="0" cellspacing="0" style="padding:32px 0;">
    <tr><td align="center">
      <table width="480" cellpadding="0" cellspacing="0" style="background:#ffffff;border-radius:12px;padding:32px;">
        <tr><td>
          <h1 style="margin:0 0 8px;font-size:20px;color:#0f172a;">Reset your password</h1>
          <p style="margin:0 0 20px;font-size:15px;color:#475569;line-height:1.5;">Hi %s,</p>
          <p style="margin:0 0 24px;font-size:15px;color:#475569;line-height:1.5;">
            We received a request to reset your password. This link expires in 1 hour.
          </p>
          <p style="margin:0 0 24px;">
            <a href="%s" style="display:inline-block;background:#0d9488;color:#ffffff;text-decoration:none;font-size:15px;font-weight:600;padding:12px 24px;border-radius:8px;">Reset password</a>
          </p>
          <p style="margin:0;font-size:13px;color:#94a3b8;line-height:1.5;">
            If you didn&apos;t request a password reset, you can safely ignore this email.<br>
            Or copy this link: <span style="word-break:break-all;">%s</span>
          </p>
        </td></tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`, name, resetURL, resetURL)
}
