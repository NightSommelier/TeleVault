package telegramauth

import (
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"github.com/NightSommelier/TeleVault/backend/internal/auth"
)

func TestMapTelegramSignInError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "invalid phone code",
			err:  tgerr.New(400, "PHONE_CODE_INVALID"),
			want: auth.ErrTelegramCodeInvalid,
		},
		{
			name: "empty phone code",
			err:  tgerr.New(400, "PHONE_CODE_EMPTY"),
			want: auth.ErrTelegramCodeInvalid,
		},
		{
			name: "expired phone code",
			err:  tgerr.New(400, "PHONE_CODE_EXPIRED"),
			want: auth.ErrTelegramCodeExpired,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapTelegramSignInError(tt.err)
			if !errors.Is(got, tt.want) {
				t.Fatalf("mapTelegramSignInError() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMapTelegramCodeSendError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantIs   error
		wantWait time.Duration
	}{
		{
			name:   "invalid phone number",
			err:    tgerr.New(400, "PHONE_NUMBER_INVALID"),
			wantIs: auth.ErrTelegramPhoneInvalid,
		},
		{
			name:   "phone flood",
			err:    tgerr.New(400, "PHONE_NUMBER_FLOOD"),
			wantIs: auth.ErrTelegramSendCodeRateLimited,
		},
		{
			name:   "phone password flood",
			err:    tgerr.New(400, "PHONE_PASSWORD_FLOOD"),
			wantIs: auth.ErrTelegramSendCodeRateLimited,
		},
		{
			name:   "invalid phone code hash",
			err:    tgerr.New(400, "PHONE_CODE_HASH_INVALID"),
			wantIs: auth.ErrInvalidChallenge,
		},
		{
			name:     "flood wait with timeout",
			err:      errors.New("FLOOD_WAIT_42"),
			wantIs:   auth.ErrTelegramSendCodeRateLimited,
			wantWait: 42 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapTelegramCodeSendError(tt.err)
			if !errors.Is(got, tt.wantIs) {
				t.Fatalf("mapTelegramCodeSendError() = %v, want %v", got, tt.wantIs)
			}
			if tt.wantWait > 0 {
				var rateLimited auth.TelegramRateLimitError
				if !errors.As(got, &rateLimited) {
					t.Fatalf("mapTelegramCodeSendError() did not return TelegramRateLimitError: %T", got)
				}
				if rateLimited.RetryAfter != tt.wantWait {
					t.Fatalf("retry after = %v, want %v", rateLimited.RetryAfter, tt.wantWait)
				}
			}
		})
	}
}

func TestMapTelegramSessionValidationError(t *testing.T) {
	tests := []struct {
		name   string
		err    error
		wantIs error
	}{
		{name: "auth key unregistered", err: tgerr.New(401, "AUTH_KEY_UNREGISTERED"), wantIs: auth.ErrTelegramSessionInvalid},
		{name: "session revoked", err: tgerr.New(401, "SESSION_REVOKED"), wantIs: auth.ErrTelegramSessionInvalid},
		{name: "session expired", err: tgerr.New(401, "SESSION_EXPIRED"), wantIs: auth.ErrTelegramSessionInvalid},
		{name: "user deactivated", err: tgerr.New(401, "USER_DEACTIVATED"), wantIs: auth.ErrTelegramSessionInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapTelegramSessionValidationError(tt.err)
			if !errors.Is(got, tt.wantIs) {
				t.Fatalf("mapTelegramSessionValidationError() = %v, want %v", got, tt.wantIs)
			}
		})
	}
}

func TestCollectTelegramUserIDs(t *testing.T) {
	target := map[int64]struct{}{}
	collectTelegramUserIDs(target, []tg.UserClass{
		&tg.User{ID: 42},
		&tg.UserEmpty{},
		&tg.User{ID: 11},
		&tg.User{ID: 42},
	})

	got := make([]int64, 0, len(target))
	for id := range target {
		got = append(got, id)
	}
	want := map[int64]struct{}{11: {}, 42: {}}
	if !reflect.DeepEqual(target, want) {
		t.Fatalf("collectTelegramUserIDs() target = %v, want %v", target, want)
	}
	if len(got) != 2 {
		t.Fatalf("collectTelegramUserIDs() unique count = %d, want 2", len(got))
	}
}

func TestMapTelegramSentCodeType(t *testing.T) {
	tests := []struct {
		name   string
		input  tg.AuthSentCodeTypeClass
		kind   string
		length int
	}{
		{name: "app", input: &tg.AuthSentCodeTypeApp{Length: 5}, kind: "app", length: 5},
		{name: "sms", input: &tg.AuthSentCodeTypeSMS{Length: 6}, kind: "sms", length: 6},
		{name: "call", input: &tg.AuthSentCodeTypeCall{Length: 7}, kind: "call", length: 7},
		{name: "flash call", input: &tg.AuthSentCodeTypeFlashCall{Pattern: "*"}, kind: "flash_call", length: 0},
		{name: "missed call", input: &tg.AuthSentCodeTypeMissedCall{Prefix: "+1", Length: 4}, kind: "missed_call", length: 4},
		{name: "fragment sms", input: &tg.AuthSentCodeTypeFragmentSMS{URL: "https://fragment.com", Length: 5}, kind: "fragment_sms", length: 5},
		{name: "firebase sms", input: &tg.AuthSentCodeTypeFirebaseSMS{Length: 5}, kind: "firebase_sms", length: 5},
		{name: "sms word", input: &tg.AuthSentCodeTypeSMSWord{}, kind: "sms_word", length: 0},
		{name: "sms phrase", input: &tg.AuthSentCodeTypeSMSPhrase{}, kind: "sms_phrase", length: 0},
		{name: "email", input: &tg.AuthSentCodeTypeEmailCode{Length: 5}, kind: "email", length: 5},
		{name: "email setup required", input: &tg.AuthSentCodeTypeSetUpEmailRequired{}, kind: "email_setup_required", length: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kind, length := mapTelegramSentCodeType(tt.input)
			if kind != tt.kind || length != tt.length {
				t.Fatalf("mapTelegramSentCodeType() = (%q, %d), want (%q, %d)", kind, length, tt.kind, tt.length)
			}
		})
	}
}

func TestMapTelegramNextCodeType(t *testing.T) {
	tests := []struct {
		name  string
		input tg.AuthCodeTypeClass
		want  string
	}{
		{name: "sms", input: &tg.AuthCodeTypeSMS{}, want: "sms"},
		{name: "call", input: &tg.AuthCodeTypeCall{}, want: "call"},
		{name: "flash call", input: &tg.AuthCodeTypeFlashCall{}, want: "flash_call"},
		{name: "missed call", input: &tg.AuthCodeTypeMissedCall{}, want: "missed_call"},
		{name: "fragment sms", input: &tg.AuthCodeTypeFragmentSMS{}, want: "fragment_sms"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapTelegramNextCodeType(tt.input); got != tt.want {
				t.Fatalf("mapTelegramNextCodeType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultClientProfile(t *testing.T) {
	profile := DefaultClientProfile()

	if profile.DeviceModel == "" || profile.SystemVersion == "" || profile.AppVersion == "" {
		t.Fatalf("default profile contains empty values: %+v", profile)
	}
	if profile.LangCode == "" || profile.SystemLangCode == "" {
		t.Fatalf("default language values are empty: %+v", profile)
	}
}

func TestClientProfileDeviceConfig(t *testing.T) {
	profile := ClientProfile{
		DeviceModel:    "Workstation",
		SystemVersion:  "NixOS 25.05",
		AppVersion:     "TeleVault/0.4.2",
		LangCode:       "uk",
		SystemLangCode: "uk-UA",
	}

	got := profile.DeviceConfig()
	want := telegram.DeviceConfig{
		DeviceModel:    "Workstation",
		SystemVersion:  "NixOS 25.05",
		AppVersion:     "TeleVault/0.4.2",
		LangCode:       "uk",
		SystemLangCode: "uk-UA",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("DeviceConfig() = %+v, want %+v", got, want)
	}
}
