// store.go
package dbmng

import (
	"context"
	"database/sql"
)

type Store struct{ db *sql.DB }

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// GET /genres?all=1
func (s *Store) ListGenres(ctx context.Context, includeDisabled bool) ([]AssetGenre, error) {
	q := `
		SELECT genre_id, genre_name, genre_code, is_disabled
		FROM asset_genres
	`
	var args []any
	if !includeDisabled {
		q += ` WHERE is_disabled = 0`
	}
	q += ` ORDER BY genre_id`

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]AssetGenre, 0, 16)
	for rows.Next() {
		var ag AssetGenre
		if err := rows.Scan(&ag.GenreID, &ag.GenreName, &ag.GenreCode, &ag.IsDisabled); err != nil {
			return nil, err
		}
		res = append(res, ag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return res, nil
}

func (s *Store) GetGenreByID(ctx context.Context, id uint) (*AssetGenre, error) {
	const q = `
		SELECT genre_id, genre_name, genre_code, is_disabled
		FROM asset_genres
		WHERE genre_id = ?
	`
	var ag AssetGenre
	err := s.db.QueryRowContext(ctx, q, id).Scan(&ag.GenreID, &ag.GenreName, &ag.GenreCode, &ag.IsDisabled)
	if err != nil {
		return nil, err
	}
	return &ag, nil
}

func (s *Store) CreateGenre(ctx context.Context, name string, code string) (*AssetGenre, error) {
	const q = `
		INSERT INTO asset_genres (genre_name, genre_code, is_disabled)
		VALUES (?, ?, 0)
	`
	r, err := s.db.ExecContext(ctx, q, name, code)
	if err != nil {
		return nil, err
	}
	lastID, err := r.LastInsertId()
	if err != nil {
		return nil, err
	}
	return &AssetGenre{
		GenreID:    uint(lastID),
		GenreName:  name,
		GenreCode:  code,
		IsDisabled: false,
	}, nil
}

func (s *Store) UpdateGenre(ctx context.Context, id uint, name string, code string, disabled bool) error {
	const q = `
		UPDATE asset_genres
		SET genre_name = ?, genre_code = ?, is_disabled = ?
		WHERE genre_id = ?
	`
	r, err := s.db.ExecContext(ctx, q, name, code, disabled, id)
	if err != nil {
		return err
	}
	aff, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if aff == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// DELETE: is_disabled=1 にする
func (s *Store) DisableGenre(ctx context.Context, id uint) error {
	const q = `
		UPDATE asset_genres
		SET is_disabled = 1
		WHERE genre_id = ?
	`
	r, err := s.db.ExecContext(ctx, q, id)
	if err != nil {
		return err
	}
	aff, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if aff == 0 {
		return sql.ErrNoRows
	}
	return nil
}
