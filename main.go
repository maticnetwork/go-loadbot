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
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
)

var RPC_SERVER string
var MNEMONIC string
var SK string
var N int
var MAX_ACCOUNTS int
var MAX_SIZE int
var DATA_PATH string

var CURRENT_ITERATIONS int = 0
var Nonce uint64 = 0
var INITIAL_SIZE int64

func main() {
	err := godotenv.Load(".env")
	if err != nil {
		log.Fatalf("Some error occured. Err: %s", err)
	}
	RPC_SERVER = os.Getenv("RPC_SERVER")
	if len(RPC_SERVER) == 0 {
		fmt.Println("Invalid RPC_SERVER flag")
		return
	}

	MNEMONIC = os.Getenv("MNEMONIC")
	if len(MNEMONIC) == 0 {
		fmt.Println("Invalid MNEMONIC flag")
		return
	}

	SK = os.Getenv("SK")
	if len(SK) == 0 {
		fmt.Println("Invalid SK flag")
		return
	}

	TPS := os.Getenv("SPEED")
	if len(TPS) == 0 {
		N = 100
	} else {
		i, err := strconv.Atoi(TPS)
		if err != nil {
			fmt.Println("Invalid TPS flag")
			return
		}
		N = i
	}

	acc := os.Getenv("MAX_ACCOUNTS")
	size := os.Getenv("MAX_SIZE")
	if len(acc) == 0 && len(size) == 0 {
		fmt.Println("Both MAX_ACCOUNTS and MAX_SIZE flags cannot be empty")
		return

	}
	if len(size) != 0 {
		i, err := strconv.Atoi(size)
		if err != nil {
			fmt.Println("Invalid MAX_SIZE flag")
			return
		}
		MAX_SIZE = i

	}
	if len(acc) != 0 {
		i, err := strconv.Atoi(acc)
		if err != nil {
			fmt.Println("Invalid MAX_ACCOUNTS flag")
			return
		}
		MAX_ACCOUNTS = i
	}

	DATA_PATH = os.Getenv("DATA_PATH")
	if len(DATA_PATH) == 0 {
		fmt.Println("Invalid DATA_PATH flag")
		return
	}

	fmt.Println("RPC_SERVER: ", RPC_SERVER)
	fmt.Println("MNEMONIC: ", MNEMONIC)
	fmt.Println("SK: ", SK)
	fmt.Println("N: ", N)
	fmt.Println("MAX_ACCOUNTS: ", MAX_ACCOUNTS)
	fmt.Println("MAX_SIZE: ", MAX_SIZE)
	fmt.Println("DATA_PATH: ", DATA_PATH)

	main1()
}

func main1() {

	fmt.Printf("script started \n")

	ctx := context.Background()

	cl, err := ethclient.Dial(RPC_SERVER)
	if err != nil {
		log.Println("Error in dial connection: ", err)
	}

	chainID, err := cl.ChainID(ctx)
	if err != nil {
		log.Println("Error in fetching chainID: ", err)
	}
	fmt.Println("Chain ID: ", chainID)

	sk := crypto.ToECDSAUnsafe(common.FromHex(SK))
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

	nonce, err := cl.PendingNonceAt(ctx, add)
	if err != nil {
		log.Fatalln("Error in getting pendingNonce: ", nonce)
	} else {
		Nonce = nonce
	}
	fmt.Println("Nonce: ", Nonce)

	generatedAccounts := generateAccountsUsingMnemonic(ctx, cl)

	fund := os.Getenv("FUND")
	if fund == "true" {
		fundAccounts(ctx, cl, generatedAccounts, chainID, add, ksOpts)
	}

	INITIAL_SIZE = checkChainData()

	fmt.Println("Preparing")
	if fund == "true" {
		fmt.Println("Loadbot Starting in 15 secs")
		time.Sleep(15 * time.Second)
	} else {
		time.Sleep(2 * time.Second)
	}

	startLoadbot(ctx, cl, chainID, generatedAccounts)

}

type Account struct {
	key  *ecdsa.PrivateKey
	addr common.Address
}

type Accounts []Account

func generateAccountsUsingMnemonic(ctx context.Context, client *ethclient.Client) (accounts Accounts) {
	wallet, err := hdwallet.NewFromMnemonic(MNEMONIC)
	if err != nil {
		log.Fatal(err)
	}

	for i := 1; i <= N; i++ {
		var dpath string = "m/44'/60'/0'/0/" + strconv.Itoa(i)
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
	senderAddress common.Address, opts *bind.TransactOpts) {
	for i := 0; i < N; i++ {
		fmt.Println("Reqd nonce: ", Nonce+uint64(i))
		runTransaction(ctx, client, genAccounts[i].addr, chainID, senderAddress, opts, Nonce+uint64(i), 10000000000000000)
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
	mu     sync.Mutex
	nonces []uint64
}

func startLoadbot(ctx context.Context, client *ethclient.Client, chainID *big.Int,
	genAccounts Accounts) {

	if MAX_SIZE > 0 {
		go func() {
			for {

				currentSize := checkChainData()
				if (currentSize - INITIAL_SIZE) > int64(MAX_SIZE) {
					fmt.Println("Size limit reached!!!")
					os.Exit(0)
				}

				time.Sleep(5 * time.Second)
			}

		}()
	}

	fmt.Printf("Loadbot started \n")
	noncesStruct := &Nonces{
		nonces: make([]uint64, N),
	}

	flag := 0
	for i, a := range genAccounts {
		if flag >= N {
			break
		}
		flag++
		fmt.Printf("i is %v \n", i)
		go func(i int, a Account, m *sync.Mutex) {
			nonce, err := client.PendingNonceAt(ctx, a.addr)
			if err != nil {
				fmt.Printf("failed to retrieve pending nonce for account %s: %v", a.addr.String(), err)
			}
			m.Lock()
			noncesStruct.nonces[i] = nonce
			m.Unlock()
		}(i, a, &noncesStruct.mu)
	}

	fmt.Printf("intialization completed \n")
	recpIdx := 0
	sendIdx := 0

	// Fire off transactions
	period := 1 * time.Second / time.Duration(N)
	ticker := time.NewTicker(period)

	for {
		select {
		case <-ticker.C:

			if CURRENT_ITERATIONS%100 == 0 && CURRENT_ITERATIONS > 0 {
				fmt.Println("CURRENT_ACCOUNTS: ", CURRENT_ITERATIONS)
			}
			if MAX_ACCOUNTS > 0 && CURRENT_ITERATIONS >= MAX_ACCOUNTS {
				os.Exit(0)
			}

			recpIdx++
			sendIdx++
			sender := genAccounts[sendIdx%N] //cfg.Accounts[sendIdx%len(cfg.Accounts)]
			nonce := noncesStruct.nonces[sendIdx%N]

			go func(sender Account, nonce uint64) error {

				recpointer := createAccount()
				recipient := recpointer.addr

				recpointer2 := createAccount()
				recipient2 := recpointer2.addr

				recpointer3 := createAccount()
				recipient3 := recpointer3.addr

				recpointer4 := createAccount()
				recipient4 := recpointer4.addr

				recpointer5 := createAccount()
				recipient5 := recpointer5.addr

				err := runBotTransaction(ctx, client, recipient, chainID, sender, nonce, 1)
				if err != nil {
					return err
				}

				err = runBotTransaction(ctx, client, recipient2, chainID, sender, nonce+1, 1)
				if err != nil {
					return err
				}

				err = runBotTransaction(ctx, client, recipient3, chainID, sender, nonce+2, 1)
				if err != nil {
					return err
				}

				err = runBotTransaction(ctx, client, recipient4, chainID, sender, nonce+3, 1)
				if err != nil {
					return err
				}

				err = runBotTransaction(ctx, client, recipient5, chainID, sender, nonce+4, 1)
				if err != nil {
					return err
				}

				return nil

			}(sender, nonce)
			noncesStruct.nonces[sendIdx%N] = noncesStruct.nonces[sendIdx%N] + 5

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

func runBotTransaction(ctx context.Context, Clients *ethclient.Client, recipient common.Address, chainID *big.Int,
	sender Account, nonce uint64, value int64) error {

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

	err = Clients.SendTransaction(ctx, signedTx)
	if err != nil {
		fmt.Printf("Error in sending tx: %s, From : %s, To : %s\n", err, sender.addr, recipient.Hash())
	}
	// Nonce++
	CURRENT_ITERATIONS++

	return err
}

func checkChainData() int64 {
	var size int64
	err := filepath.Walk(DATA_PATH, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return err
	})

	if err != nil {
		fmt.Println("Error in getting chaindata size: ", err)
		os.Exit(0)
	}
	fmt.Print("chaindata size: ", size/1024, "KB\n\n") // Originally the size is in returned in bytes

	return size / 1024
}
