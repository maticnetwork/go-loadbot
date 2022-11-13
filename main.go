// loadbot script to generate public address
// this is done by sending(celo) transactions from a single address to different addresses

package main

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"github.com/sethvargo/go-envconfig"
)

type Config struct {
	RPCServer   string `env:"RPC_SERVER,required"`
	Mnemonic    string `env:"MNEMONIC,required"`
	SK          string `env:"SK,required"`
	TPS         int    `env:"SPEED,default=100"` // fixme: it's more like TotalTxs. could we specify TPS?
	MaxAccounts int    `env:"MAX_ACCOUNTS"`
	MaxSize     int    `env:"MAX_SIZE"`
	DataPath    string `env:"DATA_PATH,required"`
	Fund        bool   `env:"FUND"`
}

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Some error occured. Err: %s", err)
	}

	var cfg Config

	if err = envconfig.Process(context.Background(), &cfg); err != nil {
		log.Fatal(err)
	}

	if cfg.MaxAccounts == 0 && cfg.MaxSize == 0 {
		fmt.Println("Both MAX_ACCOUNTS and MAX_SIZE flags cannot be empty")
		return

	}

	spew.Dump(cfg)

	run(&cfg)
}

func run(cfg *Config) {

	fmt.Printf("script started \n")

	ctx := context.Background()

	cl, err := ethclient.Dial(cfg.RPCServer)
	if err != nil {
		log.Println("Error in dial connection: ", err)
	}

	chainID, err := cl.ChainID(ctx)
	if err != nil {
		log.Println("Error in fetching chainID: ", err)
	}
	fmt.Println("Chain ID: ", chainID)

	sk := crypto.ToECDSAUnsafe(common.FromHex(cfg.SK))
	ksOpts, err := bind.NewKeyedTransactorWithChainID(sk, chainID)
	if err != nil {
		log.Println("Error in getting ksOpts: ", err)
	}
	add := crypto.PubkeyToAddress(sk.PublicKey)

	balance, err := cl.BalanceAt(ctx, add, nil)
	if err != nil {
		log.Println("Error in checking balance: ", err)
	}
	fmt.Println("Balance: ", balance)

	localNonce, err := cl.PendingNonceAt(ctx, add)
	if err != nil {
		log.Fatalln("Error in getting pendingNonce: ", localNonce)
	}

	fmt.Println("Nonce: ", localNonce)

	generatedAccounts := generateAccountsUsingMnemonic(ctx, cl, cfg)

	if cfg.Fund {
		fundAccounts(ctx, cl, generatedAccounts, chainID, add, localNonce, cfg.TPS, ksOpts)
	}

	initialSize := checkChainData(cfg)

	fmt.Println("Preparing")

	if cfg.Fund {
		fmt.Println("Loadbot Starting in 15 secs")

		time.Sleep(15 * time.Second)
	} else {
		time.Sleep(2 * time.Second)
	}

	startLoadbot(ctx, cl, chainID, generatedAccounts, cfg, initialSize)
}

type Account struct {
	key  *ecdsa.PrivateKey
	addr common.Address
}

type Accounts []Account

func generateAccountsUsingMnemonic(ctx context.Context, client *ethclient.Client, cfg *Config) (accounts Accounts) {
	wallet, err := hdwallet.NewFromMnemonic(cfg.Mnemonic)
	if err != nil {
		log.Fatal(err)
	}

	for i := 1; i <= cfg.TPS; i++ {
		var dpath = "m/44'/60'/0'/0/" + strconv.Itoa(i)

		path := hdwallet.MustParseDerivationPath(dpath)

		account, err := wallet.Derive(path, false)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Account", i, ":", account.Address)

		privKey, err := wallet.PrivateKey(account)
		if err != nil {
			log.Fatal(err)
		}

		accounts = append(accounts, Account{key: privKey, addr: account.Address})
	}

	return accounts
}

func fundAccounts(ctx context.Context, client *ethclient.Client, genAccounts Accounts, chainID *big.Int,
	senderAddress common.Address, localNonce uint64, tps int, opts *bind.TransactOpts) {
	for i := 0; i < tps; i++ {
		fmt.Println("Reqd nonce: ", localNonce+uint64(i))

		runTransaction(ctx, client, genAccounts[i].addr, chainID, senderAddress, opts, localNonce+uint64(i), 10000000000000000)
	}
}

func runTransaction(ctx context.Context, Clients *ethclient.Client, recipient common.Address, chainID *big.Int,
	senderAddress common.Address, opts *bind.TransactOpts, nonce uint64, value int64) {

	fmt.Println("Running transaction : ", nonce)
	var data []byte
	gasLimit := uint64(21000)

	gasPrice := big.NewInt(1000000000)

	val := big.NewInt(value)

	tx := types.NewTransaction(nonce, recipient, val, gasLimit, gasPrice, data)

	signedTx, err := opts.Signer(senderAddress, tx)

	if err != nil {
		log.Fatal("Error in signing tx: ", err)
	}
	err = Clients.SendTransaction(ctx, signedTx)
	if err != nil {
		log.Fatal("Error in sending tx: ", err)
	}
}

func createAccount() Account {
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		log.Fatal(err)
	}

	publicKey := privateKey.Public()
	publicKeyECDSA, ok := publicKey.(*ecdsa.PublicKey)
	if !ok {
		log.Fatal("cannot assert type: publicKey is not of type *ecdsa.PublicKey")
	}

	address := crypto.PubkeyToAddress(*publicKeyECDSA)

	account := Account{key: privateKey, addr: address}
	return account

}

type Nonces struct {
	nonces []*uint64
}

func startLoadbot(ctx context.Context, client *ethclient.Client, chainID *big.Int,
	genAccounts Accounts, cfg *Config, initialSize int64) {

	checksInProgress := new(int64)
	pendingNonceInProgress := new(int64)
	transactionInProgress := new(int64)

	if cfg.MaxSize > 0 {
		go func() {
			atomic.AddInt64(checksInProgress, 1)
			defer atomic.AddInt64(checksInProgress, -1)

			for {
				currentSize := checkChainData(cfg)
				if (currentSize - initialSize) > int64(cfg.MaxSize) {
					fmt.Println("Size limit reached!!!")
					os.Exit(0)
				}

				time.Sleep(10 * time.Second)
			}

		}()
	}

	fmt.Printf("Loadbot started \n")
	noncesStruct := &Nonces{
		nonces: make([]*uint64, cfg.TPS),
	}

	for i, a := range genAccounts[:cfg.TPS] {
		fmt.Printf("i is %v \n", i)

		go func(i int, a Account) {
			atomic.AddInt64(pendingNonceInProgress, 1)
			defer atomic.AddInt64(pendingNonceInProgress, -1)

			// fixme: why do we need it? at the beginning all nonces are 0, after starting sending transactions we are storing local nonces.
			nonce, err := client.PendingNonceAt(ctx, a.addr)
			if err != nil {
				fmt.Printf("failed to retrieve pending nonce for account %s: %v", a.addr.String(), err)
				return
			}

			atomic.StoreUint64(noncesStruct.nonces[i], nonce)
		}(i, a)
	}

	fmt.Printf("intialization completed \n")
	recpIdx := 0
	sendIdx := 0

	// Fire off transactions
	period := 1 * time.Second / time.Duration(cfg.TPS)
	ticker := time.NewTicker(period)

	iteration := new(int64)

	for {
		fmt.Printf("goroutines: transactions %d, checks %d, getting nonces %d\n",
			atomic.LoadInt64(transactionInProgress),
			atomic.LoadInt64(checksInProgress),
			atomic.LoadInt64(pendingNonceInProgress),
		)

		select {
		case <-ticker.C:
			currentIteration := atomic.LoadInt64(iteration)

			if currentIteration%100 == 0 && currentIteration > 0 {
				fmt.Println("CURRENT_ACCOUNTS: ", iteration)
			}

			if cfg.MaxAccounts > 0 && currentIteration >= int64(cfg.MaxAccounts) {
				os.Exit(0)
			}

			recpIdx++
			sendIdx++

			accountIDx := sendIdx % cfg.TPS

			sender := genAccounts[accountIDx] //cfg.Accounts[sendIdx%len(cfg.Accounts)]
			nonce := atomic.LoadUint64(noncesStruct.nonces[accountIDx])

			const repeats = 5

			// FIXME: we do nothing with the error
			go func(sender Account, nonce uint64) error {
				atomic.AddInt64(transactionInProgress, 1)
				defer atomic.AddInt64(transactionInProgress, -1)

				for i := 0; i < repeats; i++ {
					recpointer := createAccount()
					recipient := recpointer.addr

					err := runBotTransaction(ctx, client, recipient, chainID, sender, nonce+uint64(i), 1, iteration)
					if err != nil {
						return err
					}

					atomic.AddUint64(noncesStruct.nonces[accountIDx], 1)
				}

				return nil

			}(sender, nonce)

		case <-ctx.Done():
			// return group.Wait()
		}
	}
}

func genRandomGas(min int64, max int64) *big.Int {
	bg := big.NewInt(max - min)

	n, err := rand.Int(rand.Reader, bg)
	if err != nil {
		panic(err)
	}

	return big.NewInt(n.Int64() + min)
}

func runBotTransaction(ctx context.Context, clients *ethclient.Client, recipient common.Address, chainID *big.Int,
	sender Account, nonce uint64, value int64, iteration *int64) error {

	var data []byte
	gasLimit := uint64(21000)
	var gasPrice *big.Int

	r := nonce % 6
	switch r {
	case 0:
		gasPrice = genRandomGas(32000, 34000)
	case 1:
		gasPrice = genRandomGas(22000, 24000)
	case 2:
		gasPrice = genRandomGas(28000, 30000)
	case 3:
		gasPrice = genRandomGas(26000, 28000)
	case 4:
		gasPrice = genRandomGas(20000, 24000)
	case 5:
		gasPrice = genRandomGas(30000, 32000)

	}

	val := big.NewInt(value)

	tx := types.NewTransaction(nonce, recipient, val, gasLimit, gasPrice, data)

	sk := crypto.ToECDSAUnsafe(crypto.FromECDSA(sender.key)) // Sign the transaction

	opts, err := bind.NewKeyedTransactorWithChainID(sk, chainID)
	if err != nil {
		log.Fatal("Error in creating signer: ", err)
	}

	signedTx, err := opts.Signer(sender.addr, tx)
	if err != nil {
		log.Fatal("Error in signing tx: ", err)
	}

	err = clients.SendTransaction(ctx, signedTx)
	if err != nil {
		fmt.Printf("Error in sending tx: %s, From : %s, To : %s\n", err, sender.addr, recipient.Hash())
	}

	// Nonce++
	atomic.AddInt64(iteration, 1)

	return err
}

func checkChainData(cfg *Config) int64 {
	var size int64
	err := filepath.Walk(cfg.DataPath, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			size += info.Size()
		}

		return nil
	})

	if err != nil {
		fmt.Println("Error in getting chaindata size: ", err)
		os.Exit(0)
	}

	fmt.Print("chaindata size: ", size/1024, "KB\n\n") // Originally the size is in returned in bytes

	return size / 1024
}
