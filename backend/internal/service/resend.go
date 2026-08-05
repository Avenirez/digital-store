package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// ResendService sends emails via the Resend API.
type ResendService struct {
	apiKey    string
	fromEmail string
	fromName  string
	httpClient *http.Client
}

// NewResendService creates a new ResendService.
func NewResendService(apiKey, fromEmail, fromName string) *ResendService {
	return &ResendService{
		apiKey:    apiKey,
		fromEmail: fromEmail,
		fromName:  fromName,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// IsConfigured returns true if the Resend API key is set.
func (r *ResendService) IsConfigured() bool {
	return r.apiKey != ""
}

// sendEmailRequest is the JSON body for Resend's POST /emails endpoint.
type sendEmailRequest struct {
	From    string `json:"from"`
	To      []string `json:"to"`
	Subject string `json:"subject"`
	HTML    string `json:"html"`
}

// SendEmail sends an email via the Resend API.
func (r *ResendService) SendEmail(to, subject, htmlBody string) error {
	if !r.IsConfigured() {
		log.Printf("[RESEND] Not configured, skipping email to %s", to)
		return nil
	}

	from := fmt.Sprintf("%s <%s>", r.fromName, r.fromEmail)

	payload := sendEmailRequest{
		From:    from,
		To:      []string{to},
		Subject: subject,
		HTML:    htmlBody,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("resend.SendEmail: marshal error: %w", err)
	}

	req, err := http.NewRequest("POST", "https://api.resend.com/emails", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("resend.SendEmail: request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+r.apiKey)

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("resend.SendEmail: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("resend.SendEmail: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	log.Printf("[RESEND] Email sent to %s: %s", to, subject)
	return nil
}

// ─── Email Templates ─────────────────────────────────────────────

// SendRestockAlert sends a restock notification email to a subscriber.
func (r *ResendService) SendRestockAlert(to, productTitle, productURL string) error {
	subject := fmt.Sprintf("🔔 %s is back in stock!", productTitle)
	html := fmt.Sprintf(`
		<div style="font-family: 'Segoe UI', Arial, sans-serif; max-width: 600px; margin: 0 auto; background: #0f0f0f; color: #e0e0e0; border-radius: 12px; overflow: hidden;">
			<div style="background: linear-gradient(135deg, #6366f1, #8b5cf6); padding: 32px; text-align: center;">
				<h1 style="margin: 0; color: white; font-size: 24px;">Item Back in Stock! 🎉</h1>
			</div>
			<div style="padding: 32px;">
				<p style="font-size: 16px; line-height: 1.6;">Great news! <strong>%s</strong> is now available again.</p>
				<p style="font-size: 16px; line-height: 1.6;">Grab it before it runs out — stock is limited!</p>
				<div style="text-align: center; margin: 32px 0;">
					<a href="%s" style="display: inline-block; background: linear-gradient(135deg, #6366f1, #8b5cf6); color: white; padding: 14px 32px; border-radius: 8px; text-decoration: none; font-weight: 600; font-size: 16px;">View Product →</a>
				</div>
			</div>
			<div style="padding: 16px 32px; background: #1a1a1a; text-align: center; font-size: 12px; color: #666;">
				You received this because you subscribed to restock alerts.
			</div>
		</div>
	`, productTitle, productURL)

	return r.SendEmail(to, subject, html)
}

// SendCredentials sends purchased account credentials to the buyer.
func (r *ResendService) SendCredentials(to, orderNumber, productTitle string, credentials []CredentialItem) error {
	subject := fmt.Sprintf("🎉 Informasi Akun %s — Order %s", productTitle, orderNumber)

	// Build credential rows
	var rows string
	for i, cred := range credentials {
		rows += fmt.Sprintf(`
			<tr style="border-bottom: 1px solid #2a2a2a;">
				<td style="padding: 12px; color: #a0a0a0;">#%d</td>
				<td style="padding: 12px; font-family: monospace;">%s</td>
				<td style="padding: 12px; font-family: monospace;">%s</td>
			</tr>
		`, i+1, cred.Email, cred.Password)
	}

	html := fmt.Sprintf(`
		<div style="font-family: 'Segoe UI', Arial, sans-serif; max-width: 600px; margin: 0 auto; background: #0f0f0f; color: #e0e0e0; border-radius: 12px; overflow: hidden;">
			<div style="background: linear-gradient(135deg, #10b981, #059669); padding: 32px; text-align: center;">
				<h1 style="margin: 0; color: white; font-size: 24px;">Pembayaran Sukses ✅</h1>
			</div>
			<div style="padding: 32px;">
				<p style="font-size: 16px; line-height: 1.6;">Terima kasih atas pembelian Anda!</p>
				<p style="font-size: 14px; color: #a0a0a0;">Order: <strong style="color: #e0e0e0;">%s</strong> | Produk: <strong style="color: #e0e0e0;">%s</strong></p>
				<table style="width: 100%%; border-collapse: collapse; margin: 24px 0; background: #1a1a1a; border-radius: 8px; overflow: hidden;">
					<thead>
						<tr style="background: #2a2a2a;">
							<th style="padding: 12px; text-align: left; color: #a0a0a0;">#</th>
							<th style="padding: 12px; text-align: left; color: #a0a0a0;">Email</th>
							<th style="padding: 12px; text-align: left; color: #a0a0a0;">Password</th>
						</tr>
					</thead>
					<tbody>%s</tbody>
				</table>
			</div>
			<div style="padding: 16px 32px; background: #1a1a1a; text-align: center; font-size: 12px; color: #666;">
				Simpan email ini dengan baik. Jangan bagikan informasi akun Anda.
			</div>
		</div>
	`, orderNumber, productTitle, rows)

	return r.SendEmail(to, subject, html)
}

// CredentialItem represents a single account credential for email delivery.
type CredentialItem struct {
	Email    string
	Password string
}
