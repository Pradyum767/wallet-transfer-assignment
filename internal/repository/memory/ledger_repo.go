package memory

import (
	"context"

	"github.com/wallet-transfer-assignment/wallet-transfer/internal/domain"
)

type ledgerRepo struct {
	store *Store
}

func (r *ledgerRepo) CreateEntries(ctx context.Context, entries []domain.LedgerEntry) error {
	r.store.ledger = append(r.store.ledger, entries...)
	return nil
}

func (r *ledgerRepo) ListByTransfer(ctx context.Context, transferID string) ([]domain.LedgerEntry, error) {
	var result []domain.LedgerEntry
	for _, e := range r.store.ledger {
		if e.TransferID == transferID {
			result = append(result, e)
		}
	}
	return result, nil
}

func (r *ledgerRepo) ListByWallet(ctx context.Context, walletID string) ([]domain.LedgerEntry, error) {
	var result []domain.LedgerEntry
	for _, e := range r.store.ledger {
		if e.WalletID == walletID {
			result = append(result, e)
		}
	}
	return result, nil
}
