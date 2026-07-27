package userservice

import (
	"context"
	"errors"

	"github.com/Cortalo/digitalgarden-backend/internal/domain/user"
)

// User is an alias (not a new type) for the domain entity.
type User = user.User

// Repository is the port the service depends on. Infra implements it
// implicitly; the service is the consumer, so it's the one that defines
// what it needs.
type Repository interface {
	GetByGoogleSub(ctx context.Context, googleSub string) (User, error)
	CreateUser(ctx context.Context, googleSub, name, email string) (User, error)
	GetByID(ctx context.Context, id int64) (User, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// FindOrCreateByGoogle looks up a user by their Google subject, creating
// one on first login.
func (s *Service) FindOrCreateByGoogle(ctx context.Context, googleSub, email, name string) (User, error) {
	existing, err := s.repo.GetByGoogleSub(ctx, googleSub)
	if err == nil {
		return existing, nil
	}
	if !errors.Is(err, user.ErrNotFound) {
		return User{}, err
	}

	return s.repo.CreateUser(ctx, googleSub, name, email)
}

func (s *Service) Get(ctx context.Context, id int64) (User, error) {
	return s.repo.GetByID(ctx, id)
}
