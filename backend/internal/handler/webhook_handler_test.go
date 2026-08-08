package handler

import (
	"testing"
)

func TestParsePaymentAmount(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedAmt   float64
		expectedValid bool
	}{
		{
			name:          "BCA transfer formatted with comma 00 and dot",
			content:       "Transfer m-BCA Rp 50.187,00 BERHASIL. No Ref: 12345",
			expectedAmt:   50187,
			expectedValid: true,
		},
		{
			name:          "BCA transfer with trailing sentence period",
			content:       "Anda telah menerima transfer dari Budi sebesar Rp 50.187.",
			expectedAmt:   50187,
			expectedValid: true,
		},
		{
			name:          "DANA no space after Rp",
			content:       "DANA: Kamu menerima uang Rp50.187 dari BUDI",
			expectedAmt:   50187,
			expectedValid: true,
		},
		{
			name:          "GoPay QRIS Payment",
			content:       "GoPay: Transaksi QRIS Rp 50.187 sukses",
			expectedAmt:   50187,
			expectedValid: true,
		},
		{
			name:          "OVO Topup",
			content:       "OVO: Transfer Masuk Rp50.187 dari akun lain",
			expectedAmt:   50187,
			expectedValid: true,
		},
		{
			name:          "SeaBank with trailing strip",
			content:       "SeaBank: Transfer masuk Rp 50.187,- berhasil",
			expectedAmt:   50187,
			expectedValid: true,
		},
		{
			name:          "Raw nominal string without dots",
			content:       "Transfer QRIS sebesar 50187 berhasil diterima",
			expectedAmt:   50187,
			expectedValid: true,
		},
		{
			name:          "Bank Mandiri transfer with IDR keyword",
			content:       "Livin by Mandiri: Credit transfer IDR 125.450 received",
			expectedAmt:   125450,
			expectedValid: true,
		},
		{
			name:          "Non payment text",
			content:       "Halo, selamat pagi, ada promo diskon hari ini!",
			expectedAmt:   0,
			expectedValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			amt, ok := parsePaymentAmount(tt.content)
			if ok != tt.expectedValid {
				t.Errorf("parsePaymentAmount() ok = %v, expectedValid %v (content: '%s')", ok, tt.expectedValid, tt.content)
			}
			if amt != tt.expectedAmt {
				t.Errorf("parsePaymentAmount() amt = %v, expectedAmt %v (content: '%s')", amt, tt.expectedAmt, tt.content)
			}
		})
	}
}
