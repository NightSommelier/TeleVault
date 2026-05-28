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
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gotd/td/session"
	"github.com/gotd/td/telegram"
	gotdauth "github.com/gotd/td/telegram/auth"
	"github.com/gotd/td/telegram/auth/qrlogin"
	"github.com/gotd/td/telegram/downloader"
	"github.com/gotd/td/telegram/uploader"
	"github.com/gotd/td/tg"
	"github.com/gotd/td/tgerr"

	"github.com/NightSommelier/TeleVault/backend/internal/auth"
)

var floodWaitPattern = regexp.MustCompile(`(?i)FLOOD_WAIT_?(\d+)`)

func mapTelegramSignInError(err error) error {
	switch {
	case tgerr.Is(err, "PHONE_CODE_INVALID"), tgerr.Is(err, "PHONE_CODE_EMPTY"):
		return auth.ErrTelegramCodeInvalid
	case tgerr.Is(err, "PHONE_CODE_EXPIRED"):
		return auth.ErrTelegramCodeExpired
	default:
		return err
	}
}

func mapTelegramCodeSendError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case tgerr.Is(err, "PHONE_NUMBER_INVALID"):
		return auth.ErrTelegramPhoneInvalid
	case tgerr.Is(err, "PHONE_NUMBER_FLOOD"), tgerr.Is(err, "PHONE_PASSWORD_FLOOD"):
		return auth.ErrTelegramSendCodeRateLimited
	case tgerr.Is(err, "PHONE_CODE_HASH_EMPTY"), tgerr.Is(err, "PHONE_CODE_HASH_INVALID"):
		return auth.ErrInvalidChallenge
	}
	wait := parseFloodWait(err)
	if wait > 0 {
		return auth.TelegramRateLimitError{RetryAfter: wait}
	}
	return err
}

func mapTelegramSessionValidationError(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case tgerr.Is(err, "AUTH_KEY_UNREGISTERED"),
		tgerr.Is(err, "SESSION_REVOKED"),
		tgerr.Is(err, "SESSION_EXPIRED"),
		tgerr.Is(err, "USER_DEACTIVATED"),
		tgerr.Is(err, "USER_DEACTIVATED_BAN"):
		return auth.ErrTelegramSessionInvalid
	default:
		return err
	}
}

func parseFloodWait(err error) time.Duration {
	if err == nil {
		return 0
	}
	matches := floodWaitPattern.FindStringSubmatch(err.Error())
	if len(matches) != 2 {
		return 0
	}
	seconds, parseErr := strconv.Atoi(matches[1])
	if parseErr != nil || seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}

func mapTelegramSentCodeType(codeType tg.AuthSentCodeTypeClass) (string, int) {
	switch value := codeType.(type) {
	case *tg.AuthSentCodeTypeApp:
		return "app", value.GetLength()
	case *tg.AuthSentCodeTypeSMS:
		return "sms", value.GetLength()
	case *tg.AuthSentCodeTypeCall:
		return "call", value.GetLength()
	case *tg.AuthSentCodeTypeFlashCall:
		return "flash_call", 0
	case *tg.AuthSentCodeTypeMissedCall:
		return "missed_call", value.GetLength()
	case *tg.AuthSentCodeTypeEmailCode:
		return "email", value.GetLength()
	case *tg.AuthSentCodeTypeSetUpEmailRequired:
		return "email_setup_required", 0
	case *tg.AuthSentCodeTypeFragmentSMS:
		return "fragment_sms", value.GetLength()
	case *tg.AuthSentCodeTypeFirebaseSMS:
		return "firebase_sms", value.GetLength()
	case *tg.AuthSentCodeTypeSMSWord:
		return "sms_word", 0
	case *tg.AuthSentCodeTypeSMSPhrase:
		return "sms_phrase", 0
	default:
		return "unknown", 0
	}
}

func mapTelegramNextCodeType(codeType tg.AuthCodeTypeClass) string {
	switch codeType.(type) {
	case *tg.AuthCodeTypeSMS:
		return "sms"
	case *tg.AuthCodeTypeCall:
		return "call"
	case *tg.AuthCodeTypeFlashCall:
		return "flash_call"
	case *tg.AuthCodeTypeMissedCall:
		return "missed_call"
	case *tg.AuthCodeTypeFragmentSMS:
		return "fragment_sms"
	default:
		return "unknown"
	}
}

type Client struct {
	appID   int
	appHash string
	profile ClientProfile
}

type ClientProfile struct {
	DeviceModel    string
	SystemVersion  string
	AppVersion     string
	LangCode       string
	SystemLangCode string
}

func DefaultClientProfile() ClientProfile {
	return ClientProfile{
		DeviceModel:    "Desktop",
		SystemVersion:  "Linux",
		AppVersion:     "TeleVault",
		LangCode:       "en",
		SystemLangCode: "en",
	}
}

func (p ClientProfile) DeviceConfig() telegram.DeviceConfig {
	return telegram.DeviceConfig{
		DeviceModel:    p.DeviceModel,
		SystemVersion:  p.SystemVersion,
		AppVersion:     p.AppVersion,
		LangCode:       p.LangCode,
		SystemLangCode: p.SystemLangCode,
	}
}

func NewClient(appID int, appHash string) *Client {
	return NewClientWithProfile(appID, appHash, DefaultClientProfile())
}

func NewClientWithProfile(appID int, appHash string, profile ClientProfile) *Client {
	return &Client{
		appID:   appID,
		appHash: appHash,
		profile: profile,
	}
}

func (c *Client) deviceConfig() telegram.DeviceConfig {
	return c.profile.DeviceConfig()
}

func telegramCodeChallengeFromSent(sent tg.AuthSentCodeClass) (auth.TelegramCodeChallenge, error) {
	code, ok := sent.(*tg.AuthSentCode)
	if !ok {
		return auth.TelegramCodeChallenge{}, fmt.Errorf("unexpected auth code response %T", sent)
	}
	if strings.TrimSpace(code.PhoneCodeHash) == "" {
		return auth.TelegramCodeChallenge{}, errors.New("telegram returned empty phone_code_hash")
	}

	challenge := auth.TelegramCodeChallenge{
		PhoneCodeHash: code.PhoneCodeHash,
		CodeType:      "unknown",
	}
	challenge.CodeType, challenge.CodeLength = mapTelegramSentCodeType(code.Type)
	if nextCodeType, ok := code.GetNextType(); ok {
		challenge.NextCodeType = mapTelegramNextCodeType(nextCodeType)
	}
	if timeout, ok := code.GetTimeout(); ok {
		challenge.TimeoutSecs = timeout
	}
	return challenge, nil
}

func (c *Client) SendCode(ctx context.Context, phone string) (auth.TelegramCodeChallenge, error) {
	storage := &session.StorageMemory{}
	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         true,
		SessionStorage:    storage,
		UpdateHandler:     nil,
		Device:            c.deviceConfig(),
		CompressThreshold: -1,
	})

	var challenge auth.TelegramCodeChallenge
	if err := client.Run(ctx, func(ctx context.Context) error {
		sent, err := client.API().AuthSendCode(ctx, &tg.AuthSendCodeRequest{
			PhoneNumber: phone,
			APIID:       c.appID,
			APIHash:     c.appHash,
			Settings:    tg.CodeSettings{},
		})
		if err != nil {
			return mapTelegramCodeSendError(err)
		}
		challenge, err = telegramCodeChallengeFromSent(sent)
		return err
	}); err != nil {
		return auth.TelegramCodeChallenge{}, err
	}

	sessionBytes, err := storage.Bytes(nil)
	if err != nil {
		return auth.TelegramCodeChallenge{}, err
	}

	challenge.Session = base64.StdEncoding.EncodeToString(sessionBytes)
	return challenge, nil
}

func (c *Client) ResendCode(ctx context.Context, phone string, challenge auth.TelegramCodeChallenge) (auth.TelegramCodeChallenge, error) {
	sessionBytes, err := base64.StdEncoding.DecodeString(challenge.Session)
	if err != nil {
		return auth.TelegramCodeChallenge{}, err
	}

	storage := &session.StorageMemory{}
	if err := storage.StoreSession(ctx, sessionBytes); err != nil {
		return auth.TelegramCodeChallenge{}, err
	}

	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         true,
		SessionStorage:    storage,
		UpdateHandler:     nil,
		Device:            c.deviceConfig(),
		CompressThreshold: -1,
	})

	var resent auth.TelegramCodeChallenge
	if err := client.Run(ctx, func(ctx context.Context) error {
		sent, err := client.API().AuthResendCode(ctx, &tg.AuthResendCodeRequest{
			PhoneNumber:   phone,
			PhoneCodeHash: challenge.PhoneCodeHash,
		})
		if err != nil {
			return mapTelegramCodeSendError(err)
		}
		resent, err = telegramCodeChallengeFromSent(sent)
		return err
	}); err != nil {
		return auth.TelegramCodeChallenge{}, err
	}

	updatedSession, err := storage.Bytes(nil)
	if err != nil {
		return auth.TelegramCodeChallenge{}, err
	}
	resent.Session = base64.StdEncoding.EncodeToString(updatedSession)
	return resent, nil
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
		Device:            c.deviceConfig(),
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
			return mapTelegramSignInError(err)
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
		Device:            c.deviceConfig(),
		CompressThreshold: -1,
	})

	runCtx, cancel := context.WithCancel(context.Background())
	firstToken := make(chan auth.TelegramQRLoginToken, 1)
	tokenUpdates := make(chan auth.TelegramQRLoginToken, 4)
	results := make(chan auth.TelegramQRLoginResult, 1)
	passwords := make(chan auth.TelegramQRLoginPasswordAttempt, 1)
	startErr := make(chan error, 1)

	go func() {
		defer close(tokenUpdates)
		defer close(results)
		defer close(passwords)

		err := client.Run(runCtx, func(ctx context.Context) error {
			userAuth := gotdauth.NewClient(client.API(), rand.Reader, c.appID, c.appHash)
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
				if errors.Is(err, gotdauth.ErrPasswordAuthNeeded) || tgerr.Is(err, "SESSION_PASSWORD_NEEDED") {
					select {
					case results <- auth.TelegramQRLoginResult{Err: auth.ErrTelegramMFARequired}:
					case <-ctx.Done():
						return ctx.Err()
					}
					for {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case attempt, ok := <-passwords:
							if !ok {
								return errors.New("qr password channel closed")
							}
							password := strings.TrimSpace(attempt.Password)
							if password == "" {
								attempt.Result <- auth.TelegramQRLoginResult{Err: auth.ErrTelegramMFARequired}
								continue
							}
							authorization, err = userAuth.Password(ctx, password)
							if errors.Is(err, gotdauth.ErrPasswordInvalid) || tg.IsPasswordHashInvalid(err) {
								attempt.Result <- auth.TelegramQRLoginResult{Err: auth.ErrTelegramMFAInvalid}
								continue
							}
							if err != nil {
								attempt.Result <- auth.TelegramQRLoginResult{Err: err}
								return err
							}
							user, ok := authorization.User.(*tg.User)
							if !ok {
								return fmt.Errorf("unexpected qr auth user %T", authorization.User)
							}
							sessionBytes, err := storage.Bytes(nil)
							if err != nil {
								attempt.Result <- auth.TelegramQRLoginResult{Err: err}
								return err
							}
							success := auth.TelegramQRLoginResult{
								Session: base64.StdEncoding.EncodeToString(sessionBytes),
								Profile: auth.TelegramProfile{
									TelegramID:  user.ID,
									Username:    user.Username,
									DisplayName: displayName(user.FirstName, user.LastName),
								},
							}
							attempt.Result <- success
							return nil
						}
					}
				}
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
			Token:     token,
			Tokens:    tokenUpdates,
			Results:   results,
			Passwords: passwords,
			Cancel:    cancel,
		}, nil
	}
}

func (c *Client) ValidateSession(ctx context.Context, encodedSession string) error {
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
		Device:            c.deviceConfig(),
		CompressThreshold: -1,
	})

	return client.Run(ctx, func(ctx context.Context) error {
		_, err := client.API().UsersGetUsers(ctx, []tg.InputUserClass{&tg.InputUserSelf{}})
		if err != nil {
			return mapTelegramSessionValidationError(err)
		}
		return nil
	})
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
		Device:            c.deviceConfig(),
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
		Device:            c.deviceConfig(),
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
		Device:            c.deviceConfig(),
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

func (c *Client) ListKnownUserIDs(ctx context.Context, encodedSession string, storagePeer string) ([]int64, error) {
	sessionBytes, err := base64.StdEncoding.DecodeString(encodedSession)
	if err != nil {
		return nil, err
	}

	storage := &session.StorageMemory{}
	if err := storage.StoreSession(ctx, sessionBytes); err != nil {
		return nil, err
	}

	client := telegram.NewClient(c.appID, c.appHash, telegram.Options{
		NoUpdates:         true,
		SessionStorage:    storage,
		UpdateHandler:     nil,
		Device:            c.deviceConfig(),
		CompressThreshold: -1,
	})

	peer := storagePeer
	if peer == "" {
		peer = "self"
	}
	if peer != "self" {
		return nil, fmt.Errorf("unsupported telegram storage peer %q", peer)
	}

	userIDs := make(map[int64]struct{})
	if err := client.Run(ctx, func(ctx context.Context) error {
		var contactsErr error
		contacts, err := client.API().ContactsGetContacts(ctx, 0)
		if err != nil {
			contactsErr = err
		} else if modified, ok := contacts.AsModified(); ok {
			collectTelegramUserIDs(userIDs, modified.GetUsers())
		}

		var dialogsErr error
		dialogs, err := client.API().MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetPeer: &tg.InputPeerEmpty{},
			Limit:      100,
			Hash:       0,
		})
		if err != nil {
			dialogsErr = err
		} else if modified, ok := dialogs.AsModified(); ok {
			collectTelegramUserIDs(userIDs, modified.GetUsers())
		}

		if contactsErr != nil && dialogsErr != nil {
			return fmt.Errorf("contacts and dialogs lookup failed: contacts=%v dialogs=%v", contactsErr, dialogsErr)
		}
		return nil
	}); err != nil {
		return nil, err
	}

	out := make([]int64, 0, len(userIDs))
	for userID := range userIDs {
		out = append(out, userID)
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out, nil
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

func collectTelegramUserIDs(target map[int64]struct{}, users []tg.UserClass) {
	for _, user := range users {
		userID := user.GetID()
		if userID <= 0 {
			continue
		}
		target[userID] = struct{}{}
	}
}
