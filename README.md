
# Transactions Loadbot

The tool is used to perform stress testing on ethereum based blockchains. It transfers blockchain native tokens to newly created accounts, at a rapid rate, and tries to bloat the state.


## Config Variables

All the configurations are to me made in the [.env](https://github.com/maticnetwork/go-loadbot/blob/main/.env) file. Basically, we have two important flags to execute the loadbot. They are -

- MAX_ACCOUNTS - Refers to the maximum number of accounts to generate and send the transactions to.

- MAX_SIZE - Refers to maximum size of state bloat (in KB) to be produced by the transactions.

Atleast one of these two flags have to be configured, at the runtime. If both the flags are configured, then the condition that is met first, will stop the loadbot.

Other important  flags are - 
- RPC_SERVER - Refers to the json rpc server

- MNEMONIC - Refers to the mnemonic to generate the accounts

- SK - Secret key/private key of the metamask first account, having sufficient balance to carry out the loadbot transactions

- SPEED - 5 * SPEED is ~ Transactions per second emitted by loadbot. It also refers to the total accounts, that will transfer the native tokens to the newly generated accounts

- DATA_PATH - Refers to the chaindata folder path, whose size will be computed after each transaction, and the script will stop if MAX_SIZE is reached.

- FUND - Flag to fund the accounts or not (total accounts to fund = TPS). It can either have the value true or false. For running the loadbot first time, it should be kept true so as to fund all the accounts that will transfer the fund to newly created accounts.
