package telegramauth

import (
	"errors"
	"reflect"
	"testing"

	"github.com/gotd/td/tg"
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
