// Package memory provides an in-process implementation of the repository
// interfaces, backed by plain maps guarded by a mutex. It exists purely to
// make the service layer's business logic unit-testable without a running
// database.
//
// UnitOfWork.Execute holds a single coarse-grained lock for the duration of
// the callback, which correctly serializes concurrent CreateTransfer calls
// (matching the atomicity guarantee the Postgres implementation provides
// via row locks) but does not exercise real lock-ordering or deadlock
// concerns — that is covered by the Postgres integration tests instead.
package memory

import (
	"context"
	"sync"

	"github.com/wallet-transfer-assignment/wallet-transfer/internal/domain"
	"github.com/wallet-transfer-assignment/wallet-transfer/internal/repository"
)

// Store is the shared in-memory database. Create one per test (or per
// process) and derive a repository.UnitOfWork from it via NewUnitOfWork.
type Store struct {
	mu          sync.Mutex
	wallets     map[string]domain.Wallet
	transfers   map[string]domain.Transfer
	ledger      []domain.LedgerEntry
	idempotency map[string]domain.IdempotencyRecord
}

// NewStore creates an empty in-memory store.
func NewStore() *Store {
	return &Store{
		wallets:     make(map[string]domain.Wallet),
		transfers:   make(map[string]domain.Transfer),
		idempotency: make(map[string]domain.IdempotencyRecord),
	}
}

// unitOfWork is the memory-backed repository.UnitOfWork.
type unitOfWork struct {
	store *Store
}

// NewUnitOfWork returns a repository.UnitOfWork backed by store.
func NewUnitOfWork(store *Store) repository.UnitOfWork {
	return &unitOfWork{store: store}
}

func (u *unitOfWork) Execute(ctx context.Context, fn func(ctx context.Context, repos repository.Repositories) error) error {
	u.store.mu.Lock()
	defer u.store.mu.Unlock()

	// Snapshot state so a failed callback can be rolled back without
	// mutating the shared store, mirroring real transaction semantics.
	snapshot := u.store.clone()

	repos := repository.Repositories{
		Wallets:     &walletRepo{store: u.store},
		Transfers:   &transferRepo{store: u.store},
		Ledger:      &ledgerRepo{store: u.store},
		Idempotency: &idempotencyRepo{store: u.store},
	}

	if err := fn(ctx, repos); err != nil {
		u.store.restore(snapshot)
		return err
	}
	return nil
}

func (s *Store) clone() *Store {
	clone := &Store{
		wallets:     make(map[string]domain.Wallet, len(s.wallets)),
		transfers:   make(map[string]domain.Transfer, len(s.transfers)),
		idempotency: make(map[string]domain.IdempotencyRecord, len(s.idempotency)),
		ledger:      append([]domain.LedgerEntry(nil), s.ledger...),
	}
	for k, v := range s.wallets {
		clone.wallets[k] = v
	}
	for k, v := range s.transfers {
		clone.transfers[k] = v
	}
	for k, v := range s.idempotency {
		clone.idempotency[k] = v
	}
	return clone
}

func (s *Store) restore(snapshot *Store) {
	s.wallets = snapshot.wallets
	s.transfers = snapshot.transfers
	s.idempotency = snapshot.idempotency
	s.ledger = snapshot.ledger
}
