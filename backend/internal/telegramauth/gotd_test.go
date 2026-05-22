package telegramauth

import (
	"errors"
	"testing"

	"github.com/gotd/td/tgerr"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
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
