package dbmng

import (
	"context"
	"database/sql"
	"errors"
	mysql "github.com/go-sql-driver/mysql"
	"strings"
)


type Service struct {
	db    *sql.DB
	store *Store
}

func NewService(db *sql.DB) *Service { return &Service{db: db, store: NewStore(db)} }

// ===== genres =====
func parseBoolish(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes" || s == "all"
}

func normalizeGenreCode(code string) (string, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return "", ErrInvalid("code is required")
	}
	// ここは好み：大文字化したいなら strings.ToUpper(code)
	return code, nil
}

func normalizeGenreName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", ErrInvalid("name is required")
	}
	return name, nil
}

func isDuplicateKey(err error) bool {
	var me *mysql.MySQLError
	if errors.As(err, &me) {
		return me.Number == 1062
	}
	return false
}

// ===== genres =====

func (s *Service) ListGenres(ctx context.Context, all string) ([]AssetGenre, error) {
	includeDisabled := parseBoolish(all)
	return s.store.ListGenres(ctx, includeDisabled)
}

func (s *Service) GetGenre(ctx context.Context, id uint) (*AssetGenre, error) {
	ag, err := s.store.GetGenreByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound("genre not found")
		}
		return nil, ErrInternal("failed to get genre")
	}
	return ag, nil
}

func (s *Service) CreateGenre(ctx context.Context, name string, code string) (*AssetGenre, error) {
	n, err := normalizeGenreName(name)
	if err != nil {
		return nil, err
	}
	c, err := normalizeGenreCode(code)
	if err != nil {
		return nil, err
	}

	ag, err := s.store.CreateGenre(ctx, n, c)
	if err != nil {
		if isDuplicateKey(err) {
			return nil, ErrConflict("genre_code already exists")
		}
		return nil, ErrInternal("failed to create genre")
	}
	return ag, nil
}

func (s *Service) UpdateGenre(ctx context.Context, id uint, name string, code string, disabled bool) (*AssetGenre, error) {
	n, err := normalizeGenreName(name)
	if err != nil {
		return nil, err
	}
	c, err := normalizeGenreCode(code)
	if err != nil {
		return nil, err
	}

	err = s.store.UpdateGenre(ctx, id, n, c, disabled)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound("genre not found")
		}
		if isDuplicateKey(err) {
			return nil, ErrConflict("genre_code already exists")
		}
		return nil, ErrInternal("failed to update genre")
	}
	return s.GetGenre(ctx, id)
}

func (s *Service) DeleteGenre(ctx context.Context, id uint) error {
	err := s.store.DisableGenre(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return ErrNotFound("genre not found")
		}
		return ErrInternal("failed to delete genre")
	}
	return nil
}
