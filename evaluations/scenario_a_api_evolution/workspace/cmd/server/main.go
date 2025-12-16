package main

import (
	"encoding/json"
	"log"
	"net/http"

	"example.com/api/internal/product"
)

func main() {
	repo := product.NewRepo()
	handler := &ProductHandler{repo: repo}

	http.HandleFunc("/products", handler.Handle)
	log.Fatal(http.ListenAndServe(":8080", nil))
}

type ProductHandler struct {
	repo *product.Repo
}

func (h *ProductHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method == "POST" {
		var p product.Product
		if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.repo.Save(p); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
	} else if r.Method == "GET" {
		products := h.repo.List()
		json.NewEncoder(w).Encode(products)
	}
}
