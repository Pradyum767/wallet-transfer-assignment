//go:build integration

// Integration tests against a real PostgreSQL instance, verifying that the
// pgx-based repositories and row-level locking behave correctly under real
// concurrent transactions.
//
// Run with a live Postgres reachable at TEST_DATABASE_URL, e.g. via the
// bundled docker-compose.yml:
//
//	docker compose up -d postgres
//	TEST_DATABASE_URL="postgres://wallet:wallet@localhost:5432/wallet_transfer?sslmode=disable" \
//	  go test -tags=integration ./internal/repository/postgres/... -v
package postgres_test

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/wallet-transfer-assignment/wallet-transfer/internal/migrations"
	"github.com/wallet-transfer-assignment/wallet-transfer/internal/repository/postgres"
	"github.com/wallet-transfer-assignment/wallet-transfer/internal/service"
)

func newTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("TEST_DATABASE_URL not set; skipping Postgres integration tests")
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect to test database: %v", err)
	}
	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("ping test database: %v", err)
	}
	if err := migrations.Apply(ctx, pool); err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func newTestServices(t *testing.T, pool *pgxpool.Pool) (*service.TransferService, *service.WalletService) {
	t.Helper()
	uow := postgres.NewUnitOfWork(pool, 3)
	return service.NewTransferService(uow), service.NewWalletService(uow)
}

// TestPostgres_ConcurrentTransfers_NoLostUpdates fires many concurrent
// transfers between two real wallets and asserts the final balances match a
// serial execution exactly, proving SELECT ... FOR UPDATE row locking
// prevents lost updates and double spending under genuine concurrent
// Postgres transactions.
func TestPostgres_ConcurrentTransfers_NoLostUpdates(t *testing.T) {
	pool := newTestPool(t)
	transfers, wallets := newTestServices(t, pool)
	ctx := context.Background()

	fromID := "it-" + uuid.NewString()
	toID := "it-" + uuid.NewString()
	const startingBalance = 100_000
	const transferAmount = 10
	const numTransfers = 100

	if _, err := wallets.CreateWallet(ctx, service.CreateWalletInput{ID: fromID, InitialBalance: startingBalance}); err != nil {
		t.Fatalf("create from wallet: %v", err)
	}
	if _, err := wallets.CreateWallet(ctx, service.CreateWalletInput{ID: toID, InitialBalance: 0}); err != nil {
		t.Fatalf("create to wallet: %v", err)
	}

	var wg sync.WaitGroup
	errs := make([]error, numTransfers)
	for i := 0; i < numTransfers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := transfers.CreateTransfer(ctx, service.CreateTransferInput{
				IdempotencyKey: fmt.Sprintf("it-concurrent-%s-%d", fromID, i),
				FromWalletID:   fromID,
				ToWalletID:     toID,
				Amount:         transferAmount,
			})
			errs[i] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("transfer %d failed: %v", i, err)
		}
	}

	from, err := wallets.GetWallet(ctx, fromID)
	if err != nil {
		t.Fatalf("get from wallet: %v", err)
	}
	to, err := wallets.GetWallet(ctx, toID)
	if err != nil {
		t.Fatalf("get to wallet: %v", err)
	}

	wantFrom := int64(startingBalance - numTransfers*transferAmount)
	wantTo := int64(numTransfers * transferAmount)
	if from.Balance != wantFrom {
		t.Errorf("from wallet balance = %d, want %d", from.Balance, wantFrom)
	}
	if to.Balance != wantTo {
		t.Errorf("to wallet balance = %d, want %d", to.Balance, wantTo)
	}
}

// TestPostgres_IdempotentReplay_AcrossTransactions verifies that a
// duplicate request replays the original committed response rather than
// re-executing the transfer against the real database.
func TestPostgres_IdempotentReplay_AcrossTransactions(t *testing.T) {
	pool := newTestPool(t)
	transfers, wallets := newTestServices(t, pool)
	ctx := context.Background()

	fromID := "it-" + uuid.NewString()
	toID := "it-" + uuid.NewString()
	if _, err := wallets.CreateWallet(ctx, service.CreateWalletInput{ID: fromID, InitialBalance: 500}); err != nil {
		t.Fatalf("create from wallet: %v", err)
	}
	if _, err := wallets.CreateWallet(ctx, service.CreateWalletInput{ID: toID, InitialBalance: 0}); err != nil {
		t.Fatalf("create to wallet: %v", err)
	}

	in := service.CreateTransferInput{
		IdempotencyKey: "it-dup-" + uuid.NewString(),
		FromWalletID:   fromID,
		ToWalletID:     toID,
		Amount:         100,
	}

	first, err := transfers.CreateTransfer(ctx, in)
	if err != nil {
		t.Fatalf("first transfer: %v", err)
	}
	second, err := transfers.CreateTransfer(ctx, in)
	if err != nil {
		t.Fatalf("second transfer: %v", err)
	}
	if !second.Replayed {
		t.Error("expected second identical request to be replayed")
	}
	if second.Transfer.ID != first.Transfer.ID {
		t.Errorf("replayed transfer ID = %s, want %s", second.Transfer.ID, first.Transfer.ID)
	}

	from, err := wallets.GetWallet(ctx, fromID)
	if err != nil {
		t.Fatalf("get from wallet: %v", err)
	}
	if from.Balance != 400 {
		t.Errorf("from wallet balance = %d, want 400 (duplicate must not double-debit)", from.Balance)
	}
}
