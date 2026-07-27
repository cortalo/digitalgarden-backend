package postgres

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/user"
)

// userRow is the persistence object (PO): the exact shape of a
// digitalgarden_user row. Only this file knows about it; everything else
// deals in user.User.
type userRow struct {
	ID        int64  `db:"user_id"`
	GoogleSub string `db:"google_sub"`
	Name      string `db:"name"`
	Email     string `db:"email"`
}

func (r userRow) toDomain() user.User {
	return user.User{
		ID:        r.ID,
		GoogleSub: r.GoogleSub,
		Name:      r.Name,
		Email:     r.Email,
	}
}

const userColumns = "user_id, google_sub, name, email"

// CreateUser inserts a new digitalgarden_user row.
func (s *Store) CreateUser(ctx context.Context, googleSub, name, email string) (user.User, error) {
	rows, err := s.pool.Query(ctx,
		"insert into digitalgarden.digitalgarden_user (google_sub, name, email) "+
			"values ($1, $2, $3) returning "+userColumns,
		googleSub, name, email,
	)
	if err != nil {
		return user.User{}, fmt.Errorf("insert user: %w", err)
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[userRow])
	if err != nil {
		return user.User{}, fmt.Errorf("insert user: %w", err)
	}

	return row.toDomain(), nil
}

// GetByGoogleSub looks up a user by their Google subject (the stable "sub"
// claim from a verified ID Token).
func (s *Store) GetByGoogleSub(ctx context.Context, googleSub string) (user.User, error) {
	rows, err := s.pool.Query(ctx,
		"select "+userColumns+" from digitalgarden.digitalgarden_user where google_sub = $1",
		googleSub,
	)
	if err != nil {
		return user.User{}, fmt.Errorf("get user: %w", err)
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[userRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrNotFound
		}
		return user.User{}, fmt.Errorf("get user: %w", err)
	}

	return row.toDomain(), nil
}

// GetByID looks up a user by their internal ID (e.g. the "sub" claim of
// our own JWT).
func (s *Store) GetByID(ctx context.Context, id int64) (user.User, error) {
	rows, err := s.pool.Query(ctx,
		"select "+userColumns+" from digitalgarden.digitalgarden_user where user_id = $1",
		id,
	)
	if err != nil {
		return user.User{}, fmt.Errorf("get user: %w", err)
	}

	row, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[userRow])
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return user.User{}, user.ErrNotFound
		}
		return user.User{}, fmt.Errorf("get user: %w", err)
	}

	return row.toDomain(), nil
}
