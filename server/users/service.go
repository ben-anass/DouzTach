package users

import (
	"context"
	"errors"
	"strings"

	"github.com/ben-anass/DouzTach/internal/repository"
	"github.com/jackc/pgx/v5/pgtype"
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
	username string,
	nickname string,
	email string,
	password string,
	favoriteLanguage string,
) (repository.User, error) {
	username = strings.TrimSpace(username)
	nickname = strings.TrimSpace(nickname)
	email = strings.TrimSpace(strings.ToLower(email))
	favoriteLanguage = strings.TrimSpace(favoriteLanguage)

	if username == "" || nickname == "" || email == "" || password == "" {
		return repository.User{}, errors.New("all required fields are required")
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
		Username:     username,
		Nickname:     nickname,
		Email:        email,
		PasswordHash: string(passwordHash),
		FavoriteLanguage: pgtype.Text{
			String: favoriteLanguage,
			Valid:  favoriteLanguage != "",
		},
	})
	if err != nil {
		return repository.User{}, err
	}

	return user, nil
}

func (s *Service) LoginUser(
	ctx context.Context,
	email string,
	password string,
) (repository.User, error) {

	email = strings.TrimSpace(strings.ToLower(email))

	if email == "" || password == "" {
		return repository.User{}, errors.New("email and password are required")
	}

	user, err := s.queries.GetUserByEmail(ctx, email)
	if err != nil {
		return repository.User{}, errors.New("invalid email or password")
	}

	err = bcrypt.CompareHashAndPassword(
		[]byte(user.PasswordHash),
		[]byte(password),
	)
	if err != nil {
		return repository.User{}, errors.New("invalid email or password")
	}

	return user, nil
}
