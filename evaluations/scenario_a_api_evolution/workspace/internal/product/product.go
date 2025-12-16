package product

import "errors"

type Product struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Price int    `json:"price"`
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
