package main

import (
	"errors"
	"sync"
)

type update struct {
	State string `json:"state"`
}

type receipt struct {
	OrderID     string `json:"order_id"`
	AmountCents int64  `json:"amount_cents"`
}

type order struct {
	ID      string   `json:"id"`
	Email   string   `json:"email"`
	Receipt receipt  `json:"receipt"`
	Updates []update `json:"updates"`
}

type store struct {
	mu     sync.Mutex
	users  map[string]bool
	orders map[string]order
}

func newStore() *store {
	return &store{users: make(map[string]bool), orders: make(map[string]order)}
}

func (s *store) signup(email string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.users[email] = true
}

func (s *store) checkout(id, email string, amount int64) (order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.users[email] {
		return order{}, errors.New("signup required")
	}
	if existing, ok := s.orders[id]; ok {
		if existing.Email != email || existing.Receipt.AmountCents != amount {
			return order{}, errors.New("order id already used with different input")
		}
		return existing, nil
	}
	o := order{ID: id, Email: email, Receipt: receipt{OrderID: id, AmountCents: amount}, Updates: []update{{State: "checked_out"}}}
	s.orders[id] = o
	return o, nil
}

func (s *store) fulfill(id string) (order, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	if !ok {
		return order{}, errors.New("order not found")
	}
	if o.Updates[len(o.Updates)-1].State != "fulfilled" {
		o.Updates = append(o.Updates, update{State: "fulfilled"})
		s.orders[id] = o
	}
	return o, nil
}

func (s *store) get(id string) (order, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	o, ok := s.orders[id]
	return o, ok
}
