package service

import (
	"context"
	"strings"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/auth"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/domain"
	"github.com/TonnPcst/martgo-backend/internal/store"
)

type UserService struct{ store store.Store }

func NewUserService(s store.Store) *UserService { return &UserService{store: s} }

type CreateUserInput struct {
	Email    string
	Password string
	FullName string
	Role     string
}

type UpdateUserInput struct {
	FullName *string
	Role     *string
	IsActive *bool
	Password *string
}

func (s *UserService) Create(ctx context.Context, in CreateUserInput) (db.User, error) {
	if !domain.IsValidRole(in.Role) {
		return db.User{}, apperr.Validation("invalid role %q", in.Role)
	}
	hash, err := auth.HashPassword(in.Password)
	if err != nil {
		return db.User{}, err
	}
	user, err := s.store.CreateUser(ctx, db.CreateUserParams{
		Email:        normalizeEmail(in.Email),
		PasswordHash: hash,
		FullName:     strings.TrimSpace(in.FullName),
		Role:         in.Role,
	})
	return user, dbError(err, "user")
}

func (s *UserService) Get(ctx context.Context, id int64) (db.User, error) {
	user, err := s.store.GetUserByID(ctx, id)
	return user, dbError(err, "user")
}

func (s *UserService) List(ctx context.Context, limit, offset int32) ([]db.User, int64, error) {
	users, err := s.store.ListUsers(ctx, db.ListUsersParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountUsers(ctx)
	if err != nil {
		return nil, 0, err
	}
	return users, total, nil
}

func (s *UserService) Update(ctx context.Context, actor Actor, id int64, in UpdateUserInput) (db.User, error) {
	if in.Role != nil && !domain.IsValidRole(*in.Role) {
		return db.User{}, apperr.Validation("invalid role %q", *in.Role)
	}
	if actor.UserID == id {
		if in.IsActive != nil && !*in.IsActive {
			return db.User{}, apperr.Validation("you cannot deactivate your own account")
		}
		if in.Role != nil && *in.Role != actor.Role {
			return db.User{}, apperr.Validation("you cannot change your own role")
		}
	}

	params := db.UpdateUserParams{ID: id, FullName: in.FullName, Role: in.Role, IsActive: in.IsActive}
	if in.Password != nil {
		hash, err := auth.HashPassword(*in.Password)
		if err != nil {
			return db.User{}, err
		}
		params.PasswordHash = &hash
	}
	user, err := s.store.UpdateUser(ctx, params)
	return user, dbError(err, "user")
}
