package service

import (
	"context"
	"strings"

	"github.com/TonnPcst/martgo/internal/apperr"
	"github.com/TonnPcst/martgo/internal/db"
	"github.com/TonnPcst/martgo/internal/store"
)

type CategoryService struct{ store store.Store }

func NewCategoryService(s store.Store) *CategoryService { return &CategoryService{store: s} }

type CategoryInput struct {
	Name        string
	Description string
}

func (s *CategoryService) Create(ctx context.Context, in CategoryInput) (db.Category, error) {
	c, err := s.store.CreateCategory(ctx, db.CreateCategoryParams{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
	})
	return c, dbError(err, "category")
}

func (s *CategoryService) Get(ctx context.Context, id int64) (db.Category, error) {
	c, err := s.store.GetCategory(ctx, id)
	return c, dbError(err, "category")
}

func (s *CategoryService) List(ctx context.Context, limit, offset int32) ([]db.Category, int64, error) {
	items, err := s.store.ListCategories(ctx, db.ListCategoriesParams{Limit: limit, Offset: offset})
	if err != nil {
		return nil, 0, err
	}
	total, err := s.store.CountCategories(ctx)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (s *CategoryService) Update(ctx context.Context, id int64, in CategoryInput) (db.Category, error) {
	c, err := s.store.UpdateCategory(ctx, db.UpdateCategoryParams{
		ID:          id,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
	})
	return c, dbError(err, "category")
}

func (s *CategoryService) Delete(ctx context.Context, id int64) error {
	n, err := s.store.DeleteCategory(ctx, id)
	if err != nil {
		return dbError(err, "category")
	}
	if n == 0 {
		return apperr.NotFound("category not found")
	}
	return nil
}
