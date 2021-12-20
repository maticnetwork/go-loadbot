// loadbot script to generate public address
// this is done by sending(celo) transactions from a single address to different addresses

package main

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/joho/godotenv"
	hdwallet "github.com/miguelmota/go-ethereum-hdwallet"
	"golang.org/x/sync/errgroup"
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

	TPS := os.Getenv("TPS")
	if len(TPS) == 0 {
		N = 500
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

	// runTransaction(ctx, cl, accs[1].Address, chainID, accs[0], ksOpts)

	// createAccount()

	// generatedAccounts := generateAccount(ctx, cl)
	// generateAccountsUsingMnemonic(ctx, cl)
	generatedAccounts := generateAccountsUsingMnemonic(ctx, cl)

	fund := os.Getenv("FUND")
	if fund == "true" {
		fundAccounts(ctx, cl, generatedAccounts, chainID, add, ksOpts)
	}

	INITIAL_SIZE = checkChainData()

	time.Sleep(5 * time.Second)

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
		// log.Println("Balance", i, ":", getBalance(ctx, client, addr))
	}
	return accounts
}

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

	fmt.Println("Running transaction : ", nonce)
	var data []byte
	gasLimit := uint64(21000)
	// gasPrice, err := Clients.SuggestGasPrice(context.Background())
	// if err != nil {
	// 	log.Fatal(err)
	// }
	gasPrice := big.NewInt(300000000000)

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
	nonces := make([]uint64, N) //len(cfg.Accounts)
	flag := 0
	for i, a := range genAccounts {
		if flag >= N {
			break
		}
		flag++
		fmt.Printf("i is %v \n", i)
		go func(i int, a Account) {
			nonce, err := client.PendingNonceAt(ctx, a.addr)
			if err != nil {
				fmt.Errorf("failed to retrieve pending nonce for account %s: %v", a.addr.String(), err)
			}
			nonces[i] = nonce
		}(i, a)
	}

	fmt.Printf("intialization completed \n")
	recpIdx := 0
	sendIdx := 0

	// Fire off transactions
	period := 1 * time.Second / time.Duration(N)
	ticker := time.NewTicker(period)
	group, ctx := errgroup.WithContext(ctx)

	// for CURRENT_ITERATIONS < 200*N {
	for {
		select {
		case <-ticker.C:

			fmt.Println("CURRENT_ACCOUNTS: ", CURRENT_ITERATIONS)
			if MAX_ACCOUNTS > 0 && CURRENT_ITERATIONS == MAX_ACCOUNTS {
				os.Exit(0)
			}

			// checkChainDataByScript()
			if MAX_SIZE > 0 {
				currentSize := checkChainData()
				if (currentSize - INITIAL_SIZE) > int64(MAX_SIZE) {
					fmt.Println("Size limit reached!!!")
					os.Exit(0)
				}
			}

			recpointer := createAccount()
			recipient := recpointer.addr
			recpIdx++

			sendIdx++
			sender := genAccounts[sendIdx%N] //cfg.Accounts[sendIdx%len(cfg.Accounts)]
			nonce := nonces[sendIdx%N]
			nonces[sendIdx%N]++

			group.Go(func() error {
				return runBotTransaction(ctx, client, recipient, chainID, sender, nonce, 1)
			})
		case <-ctx.Done():
			// return group.Wait()
		}
	}
}

func runBotTransaction(ctx context.Context, Clients *ethclient.Client, recipient common.Address, chainID *big.Int,
	sender Account, nonce uint64, value int64) error {

	var data []byte
	gasLimit := uint64(21000)
	// gasPrice, err := Clients.SuggestGasPrice(context.Background())
	// if err != nil {
	// 	log.Fatal(err)
	// }
	gasPrice := big.NewInt(10)

	val := big.NewInt(value)

	tx := types.NewTransaction(nonce, recipient, val, gasLimit, gasPrice, data)
	// tx := types.NewTx(&types.DynamicFeeTx{
	// 	Nonce:     nonce,
	// 	Gas:       gasLimit,
	// 	To:        &recipient,
	// 	Value:     val,
	// 	GasTipCap: big.NewInt(60000000000),
	// 	GasFeeCap: big.NewInt(60000000008),
	// })

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

func checkChainDataByScript() int {
	cm, err := exec.Command("/bin/sh", "./limitSize.sh").Output()
	if err != nil {
		fmt.Println("Error in running limitSize script: ", err)
	}
	output := string(cm)            // converting byte array to string
	output = output[:len(output)-1] // removing the \n from end
	fmt.Println("chaindata size: ", len(output), output)

	size, err := strconv.Atoi(output) // size is in KB
	if err != nil {
		fmt.Println("Error in string to int conversion: ", err)
		os.Exit(2)
	}
	fmt.Println("chaindata size (int): ", size, "KB\n")

	return size
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
	fmt.Println("chaindata size: ", size/1024, "KB\n") // Originally the size is in returned in bytes

	return size / 1024
}
