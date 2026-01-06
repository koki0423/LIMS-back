package auth

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// 秘密鍵 (本番では環境変数から取得推奨) :contentReference[oaicite:5]{index=5}
var jwtSecret = []byte("your-secret-key")

var (
	ErrAlreadyExists = errors.New("already exists")
	ErrNotFound      = errors.New("not found")
)

type Service struct {
	store AccountStore
}

func NewService(db *sql.DB) *Service {
	return &Service{store: NewStore(db)}
}

type AuthService interface {
	Login(ctx context.Context, id, password string) (string, error)
	Register(ctx context.Context, id, password, role string) error
	Delete(ctx context.Context, id string) error
	ChangeID(ctx context.Context, oldID, newID string) error
}

func JWTSecret() []byte {
	return jwtSecret
}

func (s *Service) Login(ctx context.Context, id, password string) (string, error) {
	acct, err := s.store.GetByID(ctx, id)
	if err != nil {
		return "", err
	}
	if acct == nil {
		return "", errors.New("authentication failed")
	}
	if acct.IsDisabled {
		return "", errors.New("account disabled")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(acct.PasswordHash), []byte(password)); err != nil {
		return "", errors.New("authentication failed")
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"sub":  acct.ID,
		"role": acct.Role,
		"exp":  time.Now().Add(24 * time.Hour).Unix(),
	})

	tokenString, err := token.SignedString(jwtSecret)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (s *Service) Register(ctx context.Context, id, password, role string) error {
	exists, err := s.store.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if exists != nil {
		return ErrAlreadyExists
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}

	return s.store.Create(ctx, &Account{
		ID:           id,
		PasswordHash: string(hash),
		Role:         role,
		IsDisabled:   false,
	})
}

func (s *Service) Delete(ctx context.Context, id string) error {
	n, err := s.store.Delete(ctx, id)
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Service) ChangeID(ctx context.Context, oldID, newID string) error {
	// old が存在するか
	old, err := s.store.GetByID(ctx, oldID)
	if err != nil {
		return err
	}
	if old == nil {
		return ErrNotFound
	}

	// new が空いてるか
	nw, err := s.store.GetByID(ctx, newID)
	if err != nil {
		return err
	}
	if nw != nil {
		return ErrAlreadyExists
	}

	updated, err := s.store.UpdateID(ctx, oldID, newID)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrNotFound
	}
	return nil
}
