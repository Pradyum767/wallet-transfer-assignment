package memory

import (
	"context"
	"time"

	"github.com/wallet-transfer-assignment/wallet-transfer/internal/domain"
)

type walletRepo struct {
	store *Store
}

func (r *walletRepo) Create(ctx context.Context, wallet *domain.Wallet) error {
	if _, exists := r.store.wallets[wallet.ID]; exists {
		return domain.ErrWalletAlreadyExists
	}
	r.store.wallets[wallet.ID] = *wallet
	return nil
}

func (r *walletRepo) Get(ctx context.Context, id string) (*domain.Wallet, error) {
	w, ok := r.store.wallets[id]
	if !ok {
		return nil, domain.ErrWalletNotFound
	}
	return &w, nil
}

// GetForUpdate behaves identically to Get: the enclosing UnitOfWork already
// holds an exclusive lock over the whole store for the duration of the
// transaction, so no additional per-row locking is needed here.
func (r *walletRepo) GetForUpdate(ctx context.Context, id string) (*domain.Wallet, error) {
	return r.Get(ctx, id)
}

func (r *walletRepo) UpdateBalance(ctx context.Context, id string, newBalance int64) error {
	w, ok := r.store.wallets[id]
	if !ok {
		return domain.ErrWalletNotFound
	}
	w.Balance = newBalance
	w.UpdatedAt = time.Now().UTC()
	r.store.wallets[id] = w
	return nil
}
