// loadbot script to generate public address
// this is done by sending(celo) transactions from a single address to different addresses

package main

import (
	"context"
	"math/big"
	"sync"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
)

// fixme: please check this test to check number of goroutines. For me, typical log is like:
// 2022/11/13 22:30:54 goroutines: iteration 3900, transactions 0, checks 0, getting nonces 0, duration 9.995875ms
// 2022/11/13 22:30:54 goroutines: iteration 4000, transactions 0, checks 0, getting nonces 0, duration 10.192042ms
// 2022/11/13 22:30:54 goroutines: iteration 4100, transactions 0, checks 0, getting nonces 0, duration 10.183834ms
// 2022/11/13 22:30:54 goroutines: iteration 4200, transactions 0, checks 0, getting nonces 0, duration 10.1965ms
// 2022/11/13 22:30:55 goroutines: iteration 4300, transactions 0, checks 0, getting nonces 0, duration 10.180917ms
func TestRunBotTransaction(t *testing.T) {
	t.Parallel()

	chainID := big.NewInt(1)
	eth := newMockEth(chainID)

	// remove
	initialSize := int64(0)

	const (
		tps      = 100
		accounts = 10_000
	)

	cfg := &Config{
		TPS:         tps,
		MaxAccounts: accounts,
	}

	accs := genAccounts(cfg.MaxAccounts)

	_ = startLoadbot(context.Background(), eth, chainID, accs, cfg, initialSize)
}

func genAccounts(n int) []Account {
	accounts := make([]Account, 0, n)

	for i := 0; i < n; i++ {
		key, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(key.PublicKey)

		accounts = append(accounts, Account{key: key, addr: addr})
	}

	return accounts
}

type mockEth struct {
	m      sync.RWMutex
	nonces map[common.Address]uint64
	signer types.Signer
}

func newMockEth(chainID *big.Int) *mockEth {
	return &mockEth{
		nonces: make(map[common.Address]uint64),
		signer: types.LatestSignerForChainID(chainID),
	}
}

func (eth *mockEth) SendTransaction(_ context.Context, tx *types.Transaction) error {
	eth.m.Lock()
	defer eth.m.Unlock()

	addr, err := types.Sender(eth.signer, tx)
	if err != nil {
		return err
	}

	eth.nonces[addr] = tx.Nonce()

	return nil
}

func (eth *mockEth) PendingNonceAt(_ context.Context, account common.Address) (uint64, error) {
	eth.m.RLock()
	defer eth.m.RUnlock()

	return eth.nonces[account] + 1, nil
}
