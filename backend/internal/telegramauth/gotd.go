package telegramauth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"math"
	"math/big"
	"strings"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	gotdauth "github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"

	"gitrepo.pp.ua/Sommelier/TeleVault/backend/internal/auth"
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
		Device:            telegram.DeviceConfig{AppVersion: "TeleVault"},
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

func (c *Client) SignIn(ctx context.Context, request auth.TelegramLoginRequest) (string, auth.TelegramProfile, error) {
	sessionBytes, err := base64.StdEncoding.DecodeString(request.Challenge.Session)
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
		Device:            telegram.DeviceConfig{AppVersion: "TeleVault"},
		CompressThreshold: -1,
	})

	var profile auth.TelegramProfile
	if err := client.Run(ctx, func(ctx context.Context) error {
		userAuth := gotdauth.NewClient(client.API(), rand.Reader, c.appID, c.appHash)
		authorization, err := userAuth.SignIn(ctx, request.Phone, request.Code, request.Challenge.PhoneCodeHash)
		if errors.Is(err, gotdauth.ErrPasswordAuthNeeded) {
			password := strings.TrimSpace(request.Password)
			if password == "" {
				return auth.ErrTelegramMFARequired
			}
			authorization, err = userAuth.Password(ctx, password)
			if errors.Is(err, gotdauth.ErrPasswordInvalid) || tg.IsPasswordHashInvalid(err) {
				return auth.ErrTelegramMFAInvalid
			}
			if err != nil {
				return err
			}
		}
		if err != nil {
			return err
		}

		authz := authorization
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

func (c *Client) StartQRLogin(ctx context.Context) (auth.TelegramQRLoginAttempt, error) {
	storage := &session.StorageMemory{}
	dispatcher := tg.NewUpdateDispatcher()
	loggedIn := qrlogin.OnLoginToken(dispatcher)
	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         false,
		SessionStorage:    storage,
		UpdateHandler:     dispatcher,
		Device:            telegram.DeviceConfig{AppVersion: "TeleVault"},
		CompressThreshold: -1,
	})

	runCtx, cancel := context.WithCancel(context.Background())
	firstToken := make(chan auth.TelegramQRLoginToken, 1)
	tokenUpdates := make(chan auth.TelegramQRLoginToken, 4)
	results := make(chan auth.TelegramQRLoginResult, 1)
	startErr := make(chan error, 1)

	go func() {
		defer close(tokenUpdates)
		defer close(results)

		err := client.Run(runCtx, func(ctx context.Context) error {
			authorization, err := client.QR().Auth(ctx, loggedIn, func(ctx context.Context, token qrlogin.Token) error {
				converted := auth.TelegramQRLoginToken{
					URL:       token.URL(),
					ExpiresAt: token.Expires(),
				}
				select {
				case firstToken <- converted:
				default:
				}
				select {
				case tokenUpdates <- converted:
				default:
				}
				return nil
			})
			if err != nil {
				return err
			}

			user, ok := authorization.User.(*tg.User)
			if !ok {
				return fmt.Errorf("unexpected qr auth user %T", authorization.User)
			}

			sessionBytes, err := storage.Bytes(nil)
			if err != nil {
				return err
			}
			results <- auth.TelegramQRLoginResult{
				Session: base64.StdEncoding.EncodeToString(sessionBytes),
				Profile: auth.TelegramProfile{
					TelegramID:  user.ID,
					Username:    user.Username,
					DisplayName: displayName(user.FirstName, user.LastName),
				},
			}
			return nil
		})
		if err != nil {
			select {
			case startErr <- err:
			default:
			}
			select {
			case results <- auth.TelegramQRLoginResult{Err: err}:
			default:
			}
		}
	}()

	select {
	case <-ctx.Done():
		cancel()
		return auth.TelegramQRLoginAttempt{}, ctx.Err()
	case err := <-startErr:
		cancel()
		return auth.TelegramQRLoginAttempt{}, err
	case token := <-firstToken:
		return auth.TelegramQRLoginAttempt{
			Token:   token,
			Tokens:  tokenUpdates,
			Results: results,
			Cancel:  cancel,
		}, nil
	}
}

func (c *Client) UploadEncryptedPart(ctx context.Context, encodedSession string, storagePeer string, name string, mimeType string, body io.Reader) (auth.TelegramUploadResult, error) {
	sessionBytes, err := base64.StdEncoding.DecodeString(encodedSession)
	if err != nil {
		return auth.TelegramUploadResult{}, err
	}
	if strings.TrimSpace(mimeType) == "" {
		mimeType = "application/octet-stream"
	}

	storage := &session.StorageMemory{}
	if err := storage.StoreSession(ctx, sessionBytes); err != nil {
		return auth.TelegramUploadResult{}, err
	}

	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         true,
		SessionStorage:    storage,
		UpdateHandler:     nil,
		Device:            telegram.DeviceConfig{AppVersion: "TeleVault"},
		CompressThreshold: -1,
	})

	peer := storagePeer
	if peer == "" {
		peer = "self"
	}
	if peer != "self" {
		return auth.TelegramUploadResult{}, fmt.Errorf("unsupported telegram storage peer %q", peer)
	}

	var messageID int64
	if err := client.Run(ctx, func(ctx context.Context) error {
		inputFile, err := uploader.NewUploader(client.API()).FromReader(ctx, name, body)
		if err != nil {
			return err
		}

		randomID, err := randomInt64()
		if err != nil {
			return err
		}

		updates, err := client.API().MessagesSendMedia(ctx, &tg.MessagesSendMediaRequest{
			Peer: &tg.InputPeerSelf{},
			Media: &tg.InputMediaUploadedDocument{
				File:      inputFile,
				MimeType:  mimeType,
				ForceFile: true,
				Attributes: []tg.DocumentAttributeClass{
					&tg.DocumentAttributeFilename{FileName: name},
				},
			},
			Message:  "",
			RandomID: randomID,
		})
		if err != nil {
			return err
		}

		messageID, err = sentMessageID(updates)
		return err
	}); err != nil {
		return auth.TelegramUploadResult{}, err
	}

	return auth.TelegramUploadResult{
		Peer:      peer,
		MessageID: messageID,
	}, nil
}

func (c *Client) DownloadEncryptedPart(ctx context.Context, encodedSession string, storagePeer string, messageID int64, dst io.Writer) error {
	sessionBytes, err := base64.StdEncoding.DecodeString(encodedSession)
	if err != nil {
		return err
	}

	storage := &session.StorageMemory{}
	if err := storage.StoreSession(ctx, sessionBytes); err != nil {
		return err
	}

	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         true,
		SessionStorage:    storage,
		UpdateHandler:     nil,
		Device:            telegram.DeviceConfig{AppVersion: "TeleVault"},
		CompressThreshold: -1,
	})

	peer := storagePeer
	if peer == "" {
		peer = "self"
	}
	if peer != "self" {
		return fmt.Errorf("unsupported telegram storage peer %q", peer)
	}
	if messageID <= 0 || messageID > int64(math.MaxInt) {
		return fmt.Errorf("invalid telegram message id %d", messageID)
	}

	return client.Run(ctx, func(ctx context.Context) error {
		location, err := documentLocation(ctx, client.API(), int(messageID))
		if err != nil {
			return err
		}

		_, err = downloader.NewDownloader().
			Download(client.API(), location).
			Stream(ctx, dst)
		return err
	})
}

func (c *Client) DeleteEncryptedPart(ctx context.Context, encodedSession string, storagePeer string, messageID int64) error {
	sessionBytes, err := base64.StdEncoding.DecodeString(encodedSession)
	if err != nil {
		return err
	}

	storage := &session.StorageMemory{}
	if err := storage.StoreSession(ctx, sessionBytes); err != nil {
		return err
	}

	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         true,
		SessionStorage:    storage,
		UpdateHandler:     nil,
		Device:            telegram.DeviceConfig{AppVersion: "TeleVault"},
		CompressThreshold: -1,
	})

	peer := storagePeer
	if peer == "" {
		peer = "self"
	}
	if peer != "self" {
		return fmt.Errorf("unsupported telegram storage peer %q", peer)
	}
	if messageID <= 0 || messageID > int64(math.MaxInt) {
		return fmt.Errorf("invalid telegram message id %d", messageID)
	}

	return client.Run(ctx, func(ctx context.Context) error {
		_, err := client.API().MessagesDeleteMessages(ctx, &tg.MessagesDeleteMessagesRequest{
			ID:     []int{int(messageID)},
			Revoke: true,
		})
		return err
	})
}

func randomInt64() (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return 0, err
	}
	return n.Int64() + 1, nil
}

func sentMessageID(updates tg.UpdatesClass) (int64, error) {
	switch value := updates.(type) {
	case *tg.UpdateShortSentMessage:
		return int64(value.ID), nil
	case *tg.Updates:
		for _, update := range value.Updates {
			newMessage, ok := update.(*tg.UpdateNewMessage)
			if !ok {
				continue
			}
			message, ok := newMessage.Message.(*tg.Message)
			if ok {
				return int64(message.ID), nil
			}
		}
	case *tg.UpdatesCombined:
		for _, update := range value.Updates {
			newMessage, ok := update.(*tg.UpdateNewMessage)
			if !ok {
				continue
			}
			message, ok := newMessage.Message.(*tg.Message)
			if ok {
				return int64(message.ID), nil
			}
		}
	}

	return 0, fmt.Errorf("unexpected messages.sendMedia response %T", updates)
}

func documentLocation(ctx context.Context, api *tg.Client, messageID int) (*tg.InputDocumentFileLocation, error) {
	messages, err := api.MessagesGetMessages(ctx, []tg.InputMessageClass{
		&tg.InputMessageID{ID: messageID},
	})
	if err != nil {
		return nil, err
	}

	message, err := firstMessage(messages)
	if err != nil {
		return nil, err
	}

	media, ok := message.Media.(*tg.MessageMediaDocument)
	if !ok {
		return nil, fmt.Errorf("message %d does not contain document media", messageID)
	}
	document, ok := media.Document.(*tg.Document)
	if !ok {
		return nil, fmt.Errorf("message %d document has unexpected type %T", messageID, media.Document)
	}

	return &tg.InputDocumentFileLocation{
		ID:            document.ID,
		AccessHash:    document.AccessHash,
		FileReference: document.FileReference,
	}, nil
}

func firstMessage(messages tg.MessagesMessagesClass) (*tg.Message, error) {
	switch value := messages.(type) {
	case *tg.MessagesMessages:
		return firstConcreteMessage(value.Messages)
	case *tg.MessagesMessagesSlice:
		return firstConcreteMessage(value.Messages)
	case *tg.MessagesChannelMessages:
		return firstConcreteMessage(value.Messages)
	default:
		return nil, fmt.Errorf("unexpected messages.getMessages response %T", messages)
	}
}

func firstConcreteMessage(messages []tg.MessageClass) (*tg.Message, error) {
	for _, message := range messages {
		if typed, ok := message.(*tg.Message); ok {
			return typed, nil
		}
	}
	return nil, errors.New("message not found")
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
