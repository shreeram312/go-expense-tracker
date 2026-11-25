package store

import (
	"sync"
	"time"
)


type Expense struct {
	Id int64 `json:"id"`
	AmountPaise int64 `json:"amount_paise"`
	Note string `json:"note"`
	SpentOn string `json:"spent_on"`
	CreatedAt time.Time `json:"created_at"`
}


type MemoryStore struct{
	mu sync.RWMutex
	nextId int64
	expenses map[int64]Expense
}


func NewMemoryStore() *MemoryStore{
	return &MemoryStore{
		expenses: make(map[int64]Expense),
	}
}


func (s *MemoryStore) Create(e Expense) Expense{
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nextId++
	e.Id = s.nextId
	e.CreatedAt = time.Now()
	s.expenses[s.nextId] = e
	return e

}


func (s *MemoryStore) List() []Expense{
	s.mu.Lock()
	defer s.mu.Unlock()
	list:=make([]Expense,len(s.expenses))
	
	for _,expense := range s.expenses{
		list  = append(list,expense)
	}

	return list
}