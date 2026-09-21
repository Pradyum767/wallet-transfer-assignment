package router_test

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/wallet-transfer-assignment/wallet-transfer/internal/router"
	"github.com/wallet-transfer-assignment/wallet-transfer/internal/service"
	"github.com/wallet-transfer-assignment/wallet-transfer/internal/testutil"
)

func newTestRouter(t *testing.T) http.Handler {
	t.Helper()
	uow := testutil.NewMockUnitOfWork()
	logger := slog.New(slog.NewTextHandler(bytes.NewBuffer(nil), nil))
	transfers := service.NewTransferService(uow, service.WithLogger(logger))
	wallets := service.NewWalletService(uow, service.WithLogger(logger))
	return router.New(transfers, wallets, logger)
}

func doJSON(t *testing.T, r http.Handler, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode request body: %v", err)
		}
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestTransferEndpoint_EndToEnd(t *testing.T) {
	r := newTestRouter(t)

	doJSON(t, r, http.MethodPost, "/wallets", map[string]any{"id": "wallet_1", "initialBalance": 500})
	doJSON(t, r, http.MethodPost, "/wallets", map[string]any{"id": "wallet_2", "initialBalance": 0})

	body := map[string]any{
		"idempotencyKey": "http-key-1",
		"fromWalletId":   "wallet_1",
		"toWalletId":     "wallet_2",
		"amount":         100,
	}

	first := doJSON(t, r, http.MethodPost, "/transfers", body)
	if first.Code != http.StatusCreated {
		t.Fatalf("first transfer status = %d, want 201, body=%s", first.Code, first.Body.String())
	}

	second := doJSON(t, r, http.MethodPost, "/transfers", body)
	if second.Code != http.StatusCreated {
		t.Fatalf("replayed transfer status = %d, want 201, body=%s", second.Code, second.Body.String())
	}
	if second.Header().Get("Idempotent-Replayed") != "true" {
		t.Error("expected Idempotent-Replayed header on the replayed response")
	}

	balance := doJSON(t, r, http.MethodGet, "/wallets/wallet_1/balance", nil)
	var walletBody struct {
		Balance int64 `json:"balance"`
	}
	if err := json.Unmarshal(balance.Body.Bytes(), &walletBody); err != nil {
		t.Fatalf("decode balance response: %v", err)
	}
	if walletBody.Balance != 400 {
		t.Errorf("wallet_1 balance = %d, want 400 (duplicate must not double-debit)", walletBody.Balance)
	}
}

func TestTransferEndpoint_UnknownWalletReturns404(t *testing.T) {
	r := newTestRouter(t)
	doJSON(t, r, http.MethodPost, "/wallets", map[string]any{"id": "wallet_1", "initialBalance": 500})

	resp := doJSON(t, r, http.MethodPost, "/transfers", map[string]any{
		"idempotencyKey": "missing-wallet",
		"fromWalletId":   "wallet_1",
		"toWalletId":     "does-not-exist",
		"amount":         10,
	})
	if resp.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body=%s", resp.Code, resp.Body.String())
	}
}

func TestTransferEndpoint_InvalidAmountReturns400(t *testing.T) {
	r := newTestRouter(t)
	doJSON(t, r, http.MethodPost, "/wallets", map[string]any{"id": "wallet_1", "initialBalance": 500})
	doJSON(t, r, http.MethodPost, "/wallets", map[string]any{"id": "wallet_2", "initialBalance": 0})

	resp := doJSON(t, r, http.MethodPost, "/transfers", map[string]any{
		"idempotencyKey": "bad-amount",
		"fromWalletId":   "wallet_1",
		"toWalletId":     "wallet_2",
		"amount":         0,
	})
	if resp.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body=%s", resp.Code, resp.Body.String())
	}
}
