package users

import (
	"context"
	"errors"
	"strings"

	"github.com/ben-anass/DouzTach/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

type Service struct {
	queries *repository.Queries
}

func NewService(queries *repository.Queries) *Service {
	return &Service{
		queries: queries,
	}
}

func (s *Service) CreateUser(
	ctx context.Context,
	name string,
	email string,
	password string,
) (repository.User, error) {

	name = strings.TrimSpace(name)
	email = strings.TrimSpace(strings.ToLower(email))

	if name == "" || email == "" || password == "" {
		return repository.User{}, errors.New("all fields are required")
	}

	if len(password) < 8 {
		return repository.User{}, errors.New("password must contain at least 8 characters")
	}

	passwordHash, err := bcrypt.GenerateFromPassword(
		[]byte(password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return repository.User{}, err
	}

	user, err := s.queries.CreateUser(ctx, repository.CreateUserParams{
		Name:         name,
		Email:        email,
		PasswordHash: string(passwordHash),
	})
	if err != nil {
		return repository.User{}, err
	}

	return user, nil
}
