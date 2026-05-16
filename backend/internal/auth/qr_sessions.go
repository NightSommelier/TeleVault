package auth

import (
	"errors"
	"sync"
	"time"
)

var ErrQRLoginNotFound = errors.New("qr login session not found")

type qrLoginSession struct {
	id        string
	token     TelegramQRLoginToken
	results   <-chan TelegramQRLoginResult
	cancel    func()
	expiresAt time.Time
}

type qrLoginSessions struct {
	mu       sync.Mutex
	sessions map[string]*qrLoginSession
	now      func() time.Time
}

func newQRLoginSessions() *qrLoginSessions {
	return &qrLoginSessions{
		sessions: make(map[string]*qrLoginSession),
		now:      time.Now,
	}
}

func (s *qrLoginSessions) add(id string, attempt TelegramQRLoginAttempt) {
	session := &qrLoginSession{
		id:        id,
		token:     attempt.Token,
		results:   attempt.Results,
		cancel:    attempt.Cancel,
		expiresAt: attempt.Token.ExpiresAt,
	}

	s.mu.Lock()
	s.sessions[id] = session
	s.mu.Unlock()

	go func() {
		for token := range attempt.Tokens {
			s.mu.Lock()
			if current, ok := s.sessions[id]; ok {
				current.token = token
				current.expiresAt = token.ExpiresAt
			}
			s.mu.Unlock()
		}
	}()
}

func (s *qrLoginSessions) get(id string) (*qrLoginSession, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, ErrQRLoginNotFound
	}
	if !session.expiresAt.IsZero() && !session.expiresAt.After(s.now()) {
		delete(s.sessions, id)
		if session.cancel != nil {
			session.cancel()
		}
		return nil, ErrQRLoginNotFound
	}
	return session, nil
}

func (s *qrLoginSessions) remove(id string) {
	s.mu.Lock()
	session, ok := s.sessions[id]
	delete(s.sessions, id)
	s.mu.Unlock()
	if ok && session.cancel != nil {
		session.cancel()
	}
}
