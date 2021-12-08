// loadbot script to generate public address
// this is done by sending(celo) transactions from a single address to different addresses

package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"golang.org/x/sync/errgroup"
)

// var m = "voyage fiber work thrive such soccer unveil grow state first outdoor cliff" // mnemonic to generate new public address

// // TxConfig contains the options for a transaction
// type txConfig struct {
// 	Acc       env.Account
// 	Nonce     uint64
// 	Recipient common.Address
// 	Value     *big.Int
// }

// // Config represent the load bot run configuration
// type Config struct {
// 	ChainID               *big.Int
// 	Accounts              []env.Account
// 	Amount                *big.Int
// 	TransactionsPerSecond int
// 	Clients               []*ethclient.Client
// 	Verbose               bool
// 	MaxPending            uint64
// 	SkipGasEstimation     bool
// 	MixFeeCurrency        bool
// }

// Start will start loads bots

const N int = 1000

var CURRENT_ITERATIONS int = 0

var Nonce uint64 = 0

var RPC_SERVER string = "http://3.94.19.25:9545"

// var RPC_SERVER string = "http://127.0.0.1:8545"

func main() {

	fmt.Printf("script started \n")

	ctx := context.Background()

	cl, err := ethclient.Dial(RPC_SERVER)
	if err != nil {
		log.Println("Error in dial connection: ", err)
	}

	addr := common.HexToAddress("0766fd8a11485bb8c045919ac5a07c51b3d1696b")
	balance, err := cl.BalanceAt(ctx, addr, nil)
	if err != nil {
		log.Println("Error in checking balance: ", err)
	}

	fmt.Println("Balance: ", balance)

	chainID, err := cl.ChainID(ctx)
	if err != nil {
		log.Println("Error in fetching chainID: ", err)
	}
	fmt.Println("Chain ID: ", chainID)

	// SK := "7964b5b6fe2c5c7c9d2e7025dd601c4d508570e6a24cdcbaf36d70c7ddc30b10"
	SK := "bc8737a0eb799e1273883887c7fc58a3f74ce9b26a0a40229a1d0ae7feaec9b1"
	sk := crypto.ToECDSAUnsafe(common.FromHex(SK))
	ksOpts, err := bind.NewKeyedTransactorWithChainID(sk, chainID)
	if err != nil {
		log.Println("Error in getting ksOpts: ", err)
	}
	add := crypto.PubkeyToAddress(sk.PublicKey)

	// ks := keystore.NewKeyStore("/home/rajat/geth-git/test-chain-dir/keystore", keystore.StandardScryptN, keystore.StandardScryptP)
	// ks := keystore.NewKeyStore("/home/rajat/geth-git/keystore", keystore.StandardScryptN, keystore.StandardScryptP)
	// // List all accounts
	// accs := ks.Accounts()
	// ks.Unlock(accs[0], "Rajat123")
	// // Create a TransactOpts object
	// ksOpts, err := bind.NewKeyStoreTransactorWithChainID(ks, accs[0], chainID)
	// if err != nil {
	// 	log.Println("Error in getting ksOpts: ", err)
	// }

	// fmt.Println("Account1: ", accs[0])
	balance, err = cl.BalanceAt(ctx, add, nil)
	if err != nil {
		log.Println("Error in checking balance: ", err)
	}
	fmt.Println("Balance1: ", balance)

	nonce, err := cl.PendingNonceAt(ctx, add)
	if err != nil {
		log.Fatalln("Error in getting pendingNonce: ", nonce)
	} else {
		Nonce = nonce
	}
	fmt.Println("Nonce: ", Nonce)

	// runTransaction(ctx, cl, accs[1].Address, chainID, accs[0], ksOpts)

	// createAccount()

	generatedAccounts := generateAccount(ctx, cl)

	fundAccounts(ctx, cl, generatedAccounts, chainID, add, ksOpts)

	time.Sleep(10 * time.Second)

	startLoadbot(ctx, cl, chainID, generatedAccounts)

}

type Account struct {
	key  *ecdsa.PrivateKey
	addr common.Address
}

type Accounts []Account

func generateAccount(ctx context.Context, client *ethclient.Client) (accounts Accounts) {
	for i := 0; i < N; i++ {
		key, _ := crypto.GenerateKey()
		addr := crypto.PubkeyToAddress(key.PublicKey)
		accounts = append(accounts, Account{key: key, addr: addr})
		log.Println("Account", i, ":", addr)
		log.Println("Balance", i, ":", getBalance(ctx, client, addr))
	}
	return accounts
}

func getBalance(ctx context.Context, client *ethclient.Client, address common.Address) *big.Int {
	balance, err := client.BalanceAt(context.Background(), address, nil)
	if err != nil {
		log.Println("Error in checking balance: ", err)
	}
	// fmt.Println("Balance: ", balance)
	return balance
}

func fundAccounts(ctx context.Context, client *ethclient.Client, genAccounts Accounts, chainID *big.Int,
	senderAddress common.Address, opts *bind.TransactOpts) {
	for i := 0; i < N; i++ {
		fmt.Println("Reqd nonce: ", Nonce+uint64(i))
		runTransaction(ctx, client, genAccounts[i].addr, chainID, senderAddress, opts, Nonce+uint64(i), 1000000000000000000)
	}
}

func runTransaction(ctx context.Context, Clients *ethclient.Client, recipient common.Address, chainID *big.Int,
	senderAddress common.Address, opts *bind.TransactOpts, nonce uint64, value int64) {

	var data []byte
	gasLimit := uint64(21000)
	gasPrice, err := Clients.SuggestGasPrice(context.Background())
	if err != nil {
		log.Fatal(err)
	}

	val := big.NewInt(value)

	tx := types.NewTransaction(nonce, recipient, val, gasLimit, gasPrice, data)

	signedTx, err := opts.Signer(senderAddress, tx)

	// signedTx, err4 := types.SignTx(tx, types.NewEIP155Signer(chainID), txCfg.Acc.PrivateKey)
	if err != nil {
		log.Fatal("Error in signing tx: ", err)
	}
	err = Clients.SendTransaction(ctx, signedTx)
	if err != nil {
		log.Fatal("Error in sending tx: ", err)
	}
	// Nonce++
}

func createAccount() Account {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	privateKeyBytes := crypto.FromECDSA(privateKey)
	fmt.Println("Private Key:", hexutil.Encode(privateKeyBytes))

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	// publicKeyBytes := crypto.FromECDSAPub(publicKeyECDSA)
	// fmt.Println("Public Key:", hexutil.Encode(publicKeyBytes))

	address := crypto.PubkeyToAddress(*publicKeyECDSA)
	fmt.Println("Generated Address:", address)

	account := Account{key: privateKey, addr: address}
	return account

}

func startLoadbot(ctx context.Context, client *ethclient.Client, chainID *big.Int,
	genAccounts Accounts) {

	fmt.Printf("Loadbot started \n")
	nonces := make([]uint64, 1000) //len(cfg.Accounts)
	flag := 0
	for i, a := range genAccounts {
		if flag >= 1000 {
			break
		}
		flag++
		fmt.Printf("i is %v \n", i)
		nonce, err := client.PendingNonceAt(ctx, a.addr)
		if err != nil {
			fmt.Errorf("failed to retrieve pending nonce for account %s: %v", a.addr.String(), err)
		}
		nonces[i] = nonce
	}

	fmt.Printf("intialization completed \n")
	recpIdx := 0
	sendIdx := 0

	// Fire off transactions
	period := 1 * time.Second / time.Duration(N)
	ticker := time.NewTicker(period)
	group, ctx := errgroup.WithContext(ctx)

	for CURRENT_ITERATIONS < 50*N {
		select {
		case <-ticker.C:

			// recpointer, err1 := env.DeriveAccount(m, 1, recpIdx)
			// if err1 != nil {
			// 	log.Fatal(err1)
			// }
			recpointer := createAccount()
			recipient := recpointer.addr
			recpIdx++

			sendIdx++
			sender := genAccounts[sendIdx%1000] //cfg.Accounts[sendIdx%len(cfg.Accounts)]
			nonce := nonces[sendIdx%1000]
			nonces[sendIdx%1000]++

			group.Go(func() error {
				// txCfg := txConfig{
				// 	Acc:       sender,
				// 	Nonce:     nonce,
				// 	Recipient: recipient,
				// 	Value:     val,
				// }
				return runBotTransaction(ctx, client, recipient, chainID, sender, nonce, 1)
			})
		case <-ctx.Done():
			// return group.Wait()
		}
	}
}

func runBotTransaction(ctx context.Context, Clients *ethclient.Client, recipient common.Address, chainID *big.Int,
	sender Account, nonce uint64, value int64) error {

	// var data []byte
	gasLimit := uint64(21000)
	// gasPrice, err := Clients.SuggestGasPrice(context.Background())
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// gasPrice := big.NewInt(10)

	val := big.NewInt(value)

	// tx := types.NewTransaction(nonce, recipient, val, gasLimit, gasPrice, data)
	tx := types.NewTx(&types.DynamicFeeTx{
		Nonce:     nonce,
		Gas:       gasLimit,
		To:        &recipient,
		Value:     val,
		GasTipCap: big.NewInt(2),
		GasFeeCap: big.NewInt(100),
	})

	sk := crypto.ToECDSAUnsafe(crypto.FromECDSA(sender.key)) // Sign the transaction
	// signedTx, err := types.SignTx(tx, types.NewEIP155Signer(nil), sk)
	// You could also create a TransactOpts object
	opts, err := bind.NewKeyedTransactorWithChainID(sk, chainID)

	signedTx, err := opts.Signer(sender.addr, tx)

	// signedTx, err4 := types.SignTx(tx, types.NewEIP155Signer(chainID), txCfg.Acc.PrivateKey)
	if err != nil {
		log.Fatal("Error in signing tx: ", err)
	}
	err = Clients.SendTransaction(ctx, signedTx)
	if err != nil {
		log.Fatal("Error in sending tx: ", err)
	}
	// Nonce++
	CURRENT_ITERATIONS++

	return err
}
