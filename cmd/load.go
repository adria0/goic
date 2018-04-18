package cmd

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	cfg "github.com/adriamb/goic/config"
	eth "github.com/adriamb/goic/eth"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
)

var (
	client   *eth.Web3Client
	contract *eth.Contract
)

func httpGetFile(filepath string, url string) error {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func mustLoad() {

	// -- open keystore

	var err error
	var account accounts.Account

	ks := keystore.NewKeyStore(cfg.C.Keystore.Path, keystore.StandardScryptN, keystore.StandardScryptP)
	if len(ks.Accounts()) != 1 {
		panic(fmt.Sprintf("Not exact one account in keystore, was %v", len(ks.Accounts())))
	}

	account = ks.Accounts()[0]
	err = ks.Unlock(account, cfg.C.Keystore.Passwd)
	if err != nil {
		panic(err)
	}

	// -- create client

	client, err = eth.NewWeb3Client(
		cfg.C.Web3.RPCURL,
		ks,
		account,
	)
	if err != nil {
		panic(err)
	}

	client.ClientMutex = &sync.Mutex{}

	accountInfo, err := client.AccountInfo()
	if err != nil {
		panic(err)
	}

	log.Println("Parent chain account: ", accountInfo)

	// Initialize contract

	log.Println("Downloading ", cfg.C.Contracts.IPFSProxy.JSONURL)
	resp, err := http.Get(cfg.C.Contracts.IPFSProxy.JSONURL)
	if err != nil {
		panic(err)
	}

	defer resp.Body.Close()

	if cfg.C.Contracts.IPFSProxy.Address != "" {
		address := common.HexToAddress(cfg.C.Contracts.IPFSProxy.Address)
		contract, err = eth.NewContractFromJson(client, resp.Body, &address)
	} else {
		contract, err = eth.NewContractFromJson(client, resp.Body, nil)
	}

	// -- load contract

	if err != nil {
		panic(err)
	}

}
