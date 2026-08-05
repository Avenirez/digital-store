package service

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

// DuitkuService handles all interactions with the Duitku payment gateway.
type DuitkuService struct {
	merchantCode string
	apiKey       string
	isProduction bool
	callbackURL  string
	returnURL    string
	httpClient   *http.Client
}

// NewDuitkuService creates a new DuitkuService.
func NewDuitkuService(merchantCode, apiKey string, isProduction bool, callbackURL, returnURL string) *DuitkuService {
	return &DuitkuService{
		merchantCode: merchantCode,
		apiKey:       apiKey,
		isProduction: isProduction,
		callbackURL:  callbackURL,
		returnURL:    returnURL,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// baseURL returns the appropriate Duitku API base URL.
func (d *DuitkuService) baseURL() string {
	if d.isProduction {
		return "https://passport.duitku.com/webapi"
	}
	return "https://sandbox.duitku.com/webapi"
}

// ─── Create Transaction (Request Inquiry) ────────────────────────

// CreateTransactionRequest represents the body sent to Duitku's v2/inquiry endpoint.
type CreateTransactionRequest struct {
	MerchantCode    string `json:"merchantCode"`
	PaymentAmount   int64  `json:"paymentAmount"`
	PaymentMethod   string `json:"paymentMethod,omitempty"` // Empty = show all methods
	MerchantOrderID string `json:"merchantOrderId"`
	ProductDetails  string `json:"productDetails"`
	Email           string `json:"email"`
	CustomerVaName  string `json:"customerVaName"`
	CallbackURL     string `json:"callbackUrl"`
	ReturnURL       string `json:"returnUrl"`
	Signature       string `json:"signature"`
	ExpiryPeriod    int    `json:"expiryPeriod"` // Minutes
}

// CreateTransactionResponse represents Duitku's response to a transaction request.
type CreateTransactionResponse struct {
	MerchantCode  string `json:"merchantCode"`
	Reference     string `json:"reference"`
	PaymentURL    string `json:"paymentUrl"`
	VaNumber      string `json:"vaNumber"`
	QrString      string `json:"qrString"`
	Amount        string `json:"amount"`
	StatusCode    string `json:"statusCode"`
	StatusMessage string `json:"statusMessage"`
}

// CreateTransaction creates a new payment transaction with Duitku.
// Returns the payment URL that the customer should be redirected to.
func (d *DuitkuService) CreateTransaction(orderNumber string, amount int64, productDesc, email, customerName string) (*CreateTransactionResponse, error) {
	// Build signature: HMAC_SHA256(merchantCode + merchantOrderId + paymentAmount, apiKey)
	amountStr := strconv.FormatInt(amount, 10)
	stringToSign := d.merchantCode + orderNumber + amountStr
	signature := d.hmacSHA256(stringToSign)

	reqBody := CreateTransactionRequest{
		MerchantCode:    d.merchantCode,
		PaymentAmount:   amount,
		MerchantOrderID: orderNumber,
		ProductDetails:  productDesc,
		Email:           email,
		CustomerVaName:  customerName,
		CallbackURL:     d.callbackURL,
		ReturnURL:       d.returnURL,
		Signature:       signature,
		ExpiryPeriod:    15, // 15 minutes to match our auto-expire worker
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("duitku.CreateTransaction: marshal error: %w", err)
	}

	url := d.baseURL() + "/api/merchant/v2/inquiry"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("duitku.CreateTransaction: request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duitku.CreateTransaction: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duitku.CreateTransaction: read response error: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Printf("[DUITKU] CreateTransaction failed: HTTP %d, body: %s", resp.StatusCode, string(respBody))
		return nil, fmt.Errorf("duitku.CreateTransaction: HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	var result CreateTransactionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("duitku.CreateTransaction: unmarshal error: %w", err)
	}

	if result.StatusCode != "00" {
		return nil, fmt.Errorf("duitku.CreateTransaction: status %s: %s", result.StatusCode, result.StatusMessage)
	}

	log.Printf("[DUITKU] Transaction created: order=%s ref=%s paymentURL=%s", orderNumber, result.Reference, result.PaymentURL)
	return &result, nil
}

// ─── Callback Verification ───────────────────────────────────────

// CallbackPayload represents the form data sent by Duitku in a payment callback.
// Duitku sends callbacks as application/x-www-form-urlencoded POST.
type CallbackPayload struct {
	MerchantCode    string `json:"merchantCode"`
	Amount          string `json:"amount"`
	MerchantOrderID string `json:"merchantOrderId"`
	ProductDetail   string `json:"productDetail"`
	AdditionalParam string `json:"additionalParam"`
	PaymentCode     string `json:"paymentCode"`
	ResultCode      string `json:"resultCode"` // "00" = Success, "01" = Failed
	MerchantUserID  string `json:"merchantUserId"`
	Reference       string `json:"reference"`
	Signature       string `json:"signature"`
	PublisherOrderID string `json:"publisherOrderId"`
	SpUserHash      string `json:"spUserHash"`
	SettlementDate  string `json:"settlementDate"`
	IssuerCode      string `json:"issuerCode"`
	CustomerName    string `json:"customerName"`
}

// ParseCallback parses and verifies a Duitku callback from an HTTP request.
// Returns the parsed payload if signature verification passes.
func (d *DuitkuService) ParseCallback(r *http.Request) (*CallbackPayload, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("duitku.ParseCallback: parse form error: %w", err)
	}

	payload := &CallbackPayload{
		MerchantCode:    r.FormValue("merchantCode"),
		Amount:          r.FormValue("amount"),
		MerchantOrderID: r.FormValue("merchantOrderId"),
		ProductDetail:   r.FormValue("productDetail"),
		AdditionalParam: r.FormValue("additionalParam"),
		PaymentCode:     r.FormValue("paymentCode"),
		ResultCode:      r.FormValue("resultCode"),
		MerchantUserID:  r.FormValue("merchantUserId"),
		Reference:       r.FormValue("reference"),
		Signature:       r.FormValue("signature"),
		PublisherOrderID: r.FormValue("publisherOrderId"),
		SpUserHash:      r.FormValue("spUserHash"),
		SettlementDate:  r.FormValue("settlementDate"),
		IssuerCode:      r.FormValue("issuerCode"),
		CustomerName:    r.FormValue("customerName"),
	}

	// Validate required fields
	if payload.MerchantCode == "" || payload.Amount == "" || payload.MerchantOrderID == "" || payload.Signature == "" {
		return nil, fmt.Errorf("duitku.ParseCallback: missing required fields")
	}

	// Verify signature: HMAC_SHA256(merchantCode + amount + merchantOrderId, apiKey)
	stringToSign := payload.MerchantCode + payload.Amount + payload.MerchantOrderID
	expectedSignature := d.hmacSHA256(stringToSign)

	if !hmac.Equal([]byte(payload.Signature), []byte(expectedSignature)) {
		log.Printf("[DUITKU] Callback signature mismatch for order %s", payload.MerchantOrderID)
		return nil, fmt.Errorf("duitku.ParseCallback: signature verification failed")
	}

	log.Printf("[DUITKU] Callback verified: order=%s resultCode=%s ref=%s", payload.MerchantOrderID, payload.ResultCode, payload.Reference)
	return payload, nil
}

// IsSuccess returns true if the callback indicates a successful payment.
func (p *CallbackPayload) IsSuccess() bool {
	return p.ResultCode == "00"
}

// ─── Check Transaction Status ────────────────────────────────────

// CheckTransactionRequest is the body for checking transaction status.
type CheckTransactionRequest struct {
	MerchantCode    string `json:"merchantCode"`
	MerchantOrderID string `json:"merchantOrderId"`
	Signature       string `json:"signature"`
}

// CheckTransactionResponse is the Duitku response for status checks.
type CheckTransactionResponse struct {
	MerchantCode    string `json:"merchantCode"`
	MerchantOrderID string `json:"merchantOrderId"`
	Reference       string `json:"reference"`
	Amount          string `json:"amount"`
	StatusCode      string `json:"statusCode"`
	StatusMessage   string `json:"statusMessage"`
}

// CheckTransaction queries Duitku for the current status of a transaction.
func (d *DuitkuService) CheckTransaction(orderNumber string) (*CheckTransactionResponse, error) {
	// Signature for check: HMAC_SHA256(merchantCode + merchantOrderId, apiKey)
	stringToSign := d.merchantCode + orderNumber
	signature := d.hmacSHA256(stringToSign)

	reqBody := CheckTransactionRequest{
		MerchantCode:    d.merchantCode,
		MerchantOrderID: orderNumber,
		Signature:       signature,
	}

	body, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("duitku.CheckTransaction: marshal error: %w", err)
	}

	url := d.baseURL() + "/api/merchant/transactionStatus"
	req, err := http.NewRequest("POST", url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("duitku.CheckTransaction: request error: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("duitku.CheckTransaction: HTTP error: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("duitku.CheckTransaction: read response error: %w", err)
	}

	var result CheckTransactionResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("duitku.CheckTransaction: unmarshal error: %w", err)
	}

	return &result, nil
}

// ─── Helpers ─────────────────────────────────────────────────────

// hmacSHA256 computes HMAC-SHA256 and returns hex-encoded lowercase string.
func (d *DuitkuService) hmacSHA256(message string) string {
	mac := hmac.New(sha256.New, []byte(d.apiKey))
	mac.Write([]byte(message))
	return hex.EncodeToString(mac.Sum(nil))
}
