package app

import (
	"golang.org/x/crypto/bcrypt"
	"encoding/json"
	"github.com/sekaiichi/crud/pkg/customers"
	"net/http"
)

func (s *Server) handleCustomerRegistration(w http.ResponseWriter, r *http.Request) {
	var item *customers.Customer
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		errorWriter(w, http.StatusBadRequest, err)
		return
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(item.Password), bcrypt.DefaultCost)
	if err != nil {
		errorWriter(w, http.StatusInternalServerError, err)
		return
	}
	
	item.Password = string(hashed)
	customer, err := s.customerSvc.Save(r.Context(), item)
	if err != nil {
		errorWriter(w, http.StatusInternalServerError, err)
		return
	}
	respondJSON(w, customer)
}

func (s *Server) handleCustomerGetToken(w http.ResponseWriter, r *http.Request) {
	var item *struct {
		Login    string `json:"login"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		errorWriter(w, http.StatusBadRequest, err)
		return
	}

	token, err := s.customerSvc.Token(r.Context(), item.Login, item.Password)
	if err != nil {
 
		errorWriter(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, map[string]interface{}{"status": "ok", "token": token})
}

func (s *Server) handleCustomerGetProducts(w http.ResponseWriter, r *http.Request) {
	items, err := s.customerSvc.Products(r.Context())
	if err != nil {
 
		errorWriter(w, http.StatusBadRequest, err)
		return
	}
	respondJSON(w, items)
}
