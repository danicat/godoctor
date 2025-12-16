package product_test

import (
	"testing"

	"example.com/api/internal/product"
)

func TestRepo_Save_Category(t *testing.T) {
	repo := product.NewRepo()

	// 1. Valid Product
	p := product.Product{
		ID:       "1",
		Name:     "Test",
		Price:    100,
		Category: "Electronics", // New field
	}
	if err := repo.Save(p); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	// 2. Missing Category (Should fail validation)
	p2 := product.Product{
		ID:   "2",
		Name: "Bad",
	}
	if err := repo.Save(p2); err == nil {
		t.Error("Expected error for missing category, got nil")
	}
}
