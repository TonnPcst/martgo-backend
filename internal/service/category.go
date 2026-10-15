package service

import (
	"context"
	"strings"

	"github.com/TonnPcst/martgo-backend/internal/apperr"
	"github.com/TonnPcst/martgo-backend/internal/db"
	"github.com/TonnPcst/martgo-backend/internal/store"
)

type CategoryService struct{ store store.Store }

// NewCategoryService creates the category service.
//
// ENG: All data access goes through the given store.
// LAO: ສ້າງ service ສຳລັບຈັດການໝວດໝູ່.
//
// @param s - data store
// @returns the service
func NewCategoryService(s store.Store) *CategoryService { return &CategoryService{store: s} }

type CategoryInput struct {
	Name        string
	Description string
}

// Create creates a new category.
//
// ENG: Trims the name and description before saving.
// LAO: ສ້າງໝວດໝູ່ໃໝ່ (ຕັດຍະຫວ່າງຊື່ ແລະ ຄຳອະທິບາຍ).
//
// @param ctx - request context
// @param in - category name and description
// @returns the created category, or a Conflict error if the name already exists
func (s *CategoryService) Create(ctx context.Context, in CategoryInput) (db.Category, error) {
	c, err := s.store.CreateCategory(ctx, db.CreateCategoryParams{
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
	})
	return c, dbError(err, "category")
}

// Get loads a category by ID.
//
// ENG: Returns NotFound when there is no such category.
// LAO: ດຶງໝວດໝູ່ຕາມ ID.
//
// @param ctx - request context
// @param id - category ID
// @returns the category, or a NotFound error
func (s *CategoryService) Get(ctx context.Context, id int64) (db.Category, error) {
	c, err := s.store.GetCategory(ctx, id)
	return c, dbError(err, "category")
}

// List returns one page of categories and the total count.
//
// ENG: Runs the page query and the count query separately.
// LAO: ດຶງລາຍການໝວດໝູ່ແບບແບ່ງໜ້າ ພ້ອມຈຳນວນທັງໝົດ.
//
// @param ctx - request context
// @param limit - page size
// @param offset - number of rows to skip
// @returns the page items and the total number of categories
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

// Update replaces a category's name and description.
//
// ENG: Trims both fields before saving.
// LAO: ແກ້ໄຂຊື່ ແລະ ຄຳອະທິບາຍຂອງໝວດໝູ່.
//
// @param ctx - request context
// @param id - category ID
// @param in - new name and description
// @returns the updated category, NotFound, or Conflict if the name already exists
func (s *CategoryService) Update(ctx context.Context, id int64, in CategoryInput) (db.Category, error) {
	c, err := s.store.UpdateCategory(ctx, db.UpdateCategoryParams{
		ID:          id,
		Name:        strings.TrimSpace(in.Name),
		Description: strings.TrimSpace(in.Description),
	})
	return c, dbError(err, "category")
}

// Delete removes a category.
//
// ENG: Returns Conflict if products still reference it.
// LAO: ລຶບໝວດໝູ່ — ຖ້າຍັງມີສິນຄ້າອ້າງອີງຢູ່ຈະລຶບບໍ່ໄດ້.
//
// @param ctx - request context
// @param id - category ID
// @returns nil, NotFound, or Conflict
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
