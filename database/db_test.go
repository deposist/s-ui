package database

import (
	"errors"
	"testing"
)

type mockDefaultOutboundStore struct {
	hasTable       bool
	createTableErr error
	createErr      error
	createTable    int
	create         int
}

func (s *mockDefaultOutboundStore) HasTable(any) bool {
	return s.hasTable
}

func (s *mockDefaultOutboundStore) CreateTable(...any) error {
	s.createTable++
	return s.createTableErr
}

func (s *mockDefaultOutboundStore) Create(any) error {
	s.create++
	return s.createErr
}

func TestEnsureDefaultOutboundReturnsCreateTableError(t *testing.T) {
	want := errors.New("create table failed")
	store := &mockDefaultOutboundStore{createTableErr: want}

	err := ensureDefaultOutbound(store)
	if !errors.Is(err, want) {
		t.Fatalf("expected CreateTable error, got %v", err)
	}
	if store.create != 0 {
		t.Fatal("default outbound row should not be created after CreateTable failure")
	}
}

func TestEnsureDefaultOutboundReturnsCreateError(t *testing.T) {
	want := errors.New("create default outbound failed")
	store := &mockDefaultOutboundStore{createErr: want}

	err := ensureDefaultOutbound(store)
	if !errors.Is(err, want) {
		t.Fatalf("expected Create error, got %v", err)
	}
	if store.createTable != 1 || store.create != 1 {
		t.Fatalf("unexpected call counts: createTable=%d create=%d", store.createTable, store.create)
	}
}

func TestEnsureDefaultOutboundSkipsExistingTable(t *testing.T) {
	store := &mockDefaultOutboundStore{hasTable: true}

	if err := ensureDefaultOutbound(store); err != nil {
		t.Fatal(err)
	}
	if store.createTable != 0 || store.create != 0 {
		t.Fatalf("existing table should skip writes: createTable=%d create=%d", store.createTable, store.create)
	}
}
