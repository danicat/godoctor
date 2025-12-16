package product

import "errors"

type Product struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Price    int    `json:"price"`
	Category string `json:"category"`
}

type Repo struct {
	data map[string]Product
}

func NewRepo() *Repo {
	return &Repo{data: make(map[string]Product)}
}

func (r *Repo) Save(p Product) error {
	if p.ID == "" {
		return errors.New("missing ID")
	}
	if p.Category == "" {
		return errors.New("missing category")
	}
	r.data[p.ID] = p
	return nil
}

func (r *Repo) List() []Product {
	var list []Product
	for _, p := range r.data {
		list = append(list, p)
	}
	return list
}
