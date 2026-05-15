package telegramauth

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	"github.com/gotd/td/tg"

	"github.com/televault/TeleVault/backend/internal/auth"
)

type Client struct {
	appID   int
	appHash string
}

func NewClient(appID int, appHash string) *Client {
	return &Client{
		appID:   appID,
		appHash: appHash,
	}
}

func (c *Client) SendCode(ctx context.Context, phone string) (auth.TelegramCodeChallenge, error) {
	storage := &session.StorageMemory{}
	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         true,
		SessionStorage:    storage,
		UpdateHandler:     nil,
		Device:            telegram.DeviceConfig{AppVersion: "TeleDrive 2.0"},
		CompressThreshold: -1,
	})

	var phoneCodeHash string
	if err := client.Run(ctx, func(ctx context.Context) error {
		sent, err := client.API().AuthSendCode(ctx, &tg.AuthSendCodeRequest{
			PhoneNumber: phone,
			APIID:       c.appID,
			APIHash:     c.appHash,
			Settings:    tg.CodeSettings{},
		})
		if err != nil {
			return err
		}

		code, ok := sent.(*tg.AuthSentCode)
		if !ok {
			return fmt.Errorf("unexpected auth.sendCode response %T", sent)
		}

		phoneCodeHash = code.PhoneCodeHash
		return nil
	}); err != nil {
		return auth.TelegramCodeChallenge{}, err
	}
	if phoneCodeHash == "" {
		return auth.TelegramCodeChallenge{}, errors.New("telegram returned empty phone_code_hash")
	}

	sessionBytes, err := storage.Bytes(nil)
	if err != nil {
		return auth.TelegramCodeChallenge{}, err
	}

	return auth.TelegramCodeChallenge{
		PhoneCodeHash: phoneCodeHash,
		Session:       base64.StdEncoding.EncodeToString(sessionBytes),
	}, nil
}

func (c *Client) SignIn(ctx context.Context, phone string, code string, challenge auth.TelegramCodeChallenge) (string, auth.TelegramProfile, error) {
	sessionBytes, err := base64.StdEncoding.DecodeString(challenge.Session)
	if err != nil {
		return "", auth.TelegramProfile{}, err
	}

	storage := &session.StorageMemory{}
	if err := storage.StoreSession(ctx, sessionBytes); err != nil {
		return "", auth.TelegramProfile{}, err
	}

	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         true,
		SessionStorage:    storage,
		UpdateHandler:     nil,
		Device:            telegram.DeviceConfig{AppVersion: "TeleDrive 2.0"},
		CompressThreshold: -1,
	})

	var profile auth.TelegramProfile
	if err := client.Run(ctx, func(ctx context.Context) error {
		authorization, err := client.API().AuthSignIn(ctx, &tg.AuthSignInRequest{
			PhoneNumber:   phone,
			PhoneCodeHash: challenge.PhoneCodeHash,
			PhoneCode:     code,
		})
		if err != nil {
			return err
		}

		authz, ok := authorization.(*tg.AuthAuthorization)
		if !ok {
			return fmt.Errorf("unexpected auth.signIn response %T", authorization)
		}

		user, ok := authz.User.(*tg.User)
		if !ok {
			return fmt.Errorf("unexpected auth.signIn user %T", authz.User)
		}

		profile = auth.TelegramProfile{
			TelegramID:  user.ID,
			Username:    user.Username,
			DisplayName: displayName(user.FirstName, user.LastName),
		}
		return nil
	}); err != nil {
		return "", auth.TelegramProfile{}, err
	}

	sessionBytes, err = storage.Bytes(nil)
	if err != nil {
		return "", auth.TelegramProfile{}, err
	}

	return base64.StdEncoding.EncodeToString(sessionBytes), profile, nil
}

func displayName(firstName string, lastName string) string {
	switch {
	case firstName != "" && lastName != "":
		return firstName + " " + lastName
	case firstName != "":
		return firstName
	default:
		return lastName
	}
}
