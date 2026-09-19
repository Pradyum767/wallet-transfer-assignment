package memory

import (
	"context"

	"github.com/wallet-transfer-assignment/wallet-transfer/internal/domain"
)

type transferRepo struct {
	store *Store
}

func (r *transferRepo) Create(ctx context.Context, t *domain.Transfer) error {
	for _, existing := range r.store.transfers {
		if existing.IdempotencyKey == t.IdempotencyKey {
			return domain.ErrIdempotencyKeyConflict
		}
	}
	r.store.transfers[t.ID] = *t
	return nil
}

func (r *transferRepo) UpdateState(ctx context.Context, id string, state domain.TransferState, failureReason string) error {
	t, ok := r.store.transfers[id]
	if !ok {
		return domain.ErrTransferNotFound
	}
	t.State = state
	t.FailureReason = failureReason
	r.store.transfers[id] = t
	return nil
}

func (r *transferRepo) GetByID(ctx context.Context, id string) (*domain.Transfer, error) {
	t, ok := r.store.transfers[id]
	if !ok {
		return nil, domain.ErrTransferNotFound
	}
	return &t, nil
}

func (r *transferRepo) ListByWallet(ctx context.Context, walletID string) ([]domain.Transfer, error) {
	var result []domain.Transfer
	for _, t := range r.store.transfers {
		if t.FromWalletID == walletID || t.ToWalletID == walletID {
			result = append(result, t)
		}
	}
	return result, nil
}
