package adminusers

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
)

var ErrUserNotFound = errors.New("user not found")

type User struct {
	ID          string
	TelegramID  int64
	Username    sql.NullString
	DisplayName sql.NullString
	Role        string
}

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) List(ctx context.Context) ([]User, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT id, telegram_id, username, display_name, role
FROM users
ORDER BY created_at DESC, telegram_id DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var user User
		if err := rows.Scan(&user.ID, &user.TelegramID, &user.Username, &user.DisplayName, &user.Role); err != nil {
			return nil, err
		}
		users = append(users, user)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	return users, nil
}

func (s *Store) PromoteByTelegramID(ctx context.Context, telegramID int64) (User, error) {
	return s.setRoleByTelegramID(ctx, telegramID, "admin")
}

func (s *Store) DemoteByTelegramID(ctx context.Context, telegramID int64) (User, error) {
	return s.setRoleByTelegramID(ctx, telegramID, "user")
}

func (s *Store) setRoleByTelegramID(ctx context.Context, telegramID int64, role string) (User, error) {
	var user User
	err := s.db.QueryRowContext(ctx, `
UPDATE users
SET role = $2, updated_at = now()
WHERE telegram_id = $1
RETURNING id, telegram_id, username, display_name, role`,
		telegramID,
		role,
	).Scan(&user.ID, &user.TelegramID, &user.Username, &user.DisplayName, &user.Role)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, fmt.Errorf("%w: telegram_id=%d", ErrUserNotFound, telegramID)
	}
	if err != nil {
		return User{}, err
	}

	return user, nil
}
