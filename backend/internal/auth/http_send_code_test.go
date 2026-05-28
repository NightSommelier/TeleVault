package auth

import (
	"errors"
	"net/http"
	"testing"
	"time"
)

func TestNormalizeTelegramPhone(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantPhone     string
		wantErrorCode string
	}{
		{
			name:      "full e164",
			input:     "+380991112233",
			wantPhone: "+380991112233",
		},
		{
			name:      "digits only",
			input:     "380991112233",
			wantPhone: "+380991112233",
		},
		{
			name:      "with separators",
			input:     "+380 (99) 111-22-33",
			wantPhone: "+380991112233",
		},
		{
			name:          "empty",
			input:         " ",
			wantErrorCode: "phone_required",
		},
		{
			name:          "too short",
			input:         "+1234",
			wantErrorCode: "phone_invalid_format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotPhone, gotCode := normalizeTelegramPhone(tt.input)
			if gotPhone != tt.wantPhone || gotCode != tt.wantErrorCode {
				t.Fatalf("normalizeTelegramPhone() = (%q, %q), want (%q, %q)", gotPhone, gotCode, tt.wantPhone, tt.wantErrorCode)
			}
		})
	}
}

func TestClassifyTelegramCodeSendError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		fallback   string
		wantStatus int
		wantCode   string
		wantRetry  string
	}{
		{
			name:       "phone invalid",
			err:        ErrTelegramPhoneInvalid,
			fallback:   "telegram_send_code_failed",
			wantStatus: http.StatusBadRequest,
			wantCode:   "phone_invalid_format",
		},
		{
			name:       "invalid challenge",
			err:        ErrInvalidChallenge,
			fallback:   "telegram_resend_code_failed",
			wantStatus: http.StatusUnauthorized,
			wantCode:   "invalid_auth_challenge",
		},
		{
			name:       "telegram rate limited with timeout",
			err:        TelegramRateLimitError{RetryAfter: 42 * time.Second},
			fallback:   "telegram_send_code_failed",
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "auth_rate_limited",
			wantRetry:  "42",
		},
		{
			name:       "telegram rate limited without timeout",
			err:        ErrTelegramSendCodeRateLimited,
			fallback:   "telegram_send_code_failed",
			wantStatus: http.StatusTooManyRequests,
			wantCode:   "auth_rate_limited",
		},
		{
			name:       "fallback",
			err:        errors.New("boom"),
			fallback:   "telegram_resend_code_failed",
			wantStatus: http.StatusBadGateway,
			wantCode:   "telegram_resend_code_failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotStatus, gotCode, gotRetry := classifyTelegramCodeSendError(tt.err, tt.fallback)
			if gotStatus != tt.wantStatus || gotCode != tt.wantCode || gotRetry != tt.wantRetry {
				t.Fatalf("classifyTelegramCodeSendError() = (%d, %q, %q), want (%d, %q, %q)", gotStatus, gotCode, gotRetry, tt.wantStatus, tt.wantCode, tt.wantRetry)
			}
		})
	}
}
