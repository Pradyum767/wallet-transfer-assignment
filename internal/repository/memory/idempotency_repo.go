package memory

import (
	"context"
	"time"

	"github.com/wallet-transfer-assignment/wallet-transfer/internal/domain"
)

type idempotencyRepo struct {
	store *Store
}

func (r *idempotencyRepo) Claim(ctx context.Context, key, requestHash string) (bool, *domain.IdempotencyRecord, error) {
	if existing, ok := r.store.idempotency[key]; ok {
		return false, &existing, nil
	}
	r.store.idempotency[key] = domain.IdempotencyRecord{
		Key:         key,
		RequestHash: requestHash,
		CreatedAt:   time.Now().UTC(),
	}
	return true, nil, nil
}

func (r *idempotencyRepo) Complete(ctx context.Context, key, transferID string, responseStatus int, responseBody []byte) error {
	rec, ok := r.store.idempotency[key]
	if !ok {
		return domain.ErrTransferNotFound
	}
	rec.TransferID = transferID
	rec.ResponseStatus = responseStatus
	rec.ResponseBody = responseBody
	now := time.Now().UTC()
	rec.CompletedAt = &now
	r.store.idempotency[key] = rec
	return nil
}
