package cmd

import (
	"os"
	"net/http"
	"io"
	"sync"
	"fmt"

	cfg "github.com/adriamb/goic/config"
	eth "github.com/adriamb/goic/eth"
	storagedb "github.com/adriamb/goic/storage"
	log "github.com/sirupsen/logrus"

	"github.com/ethereum/go-ethereum/accounts"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	
	shell "github.com/ipfs/go-ipfs-api"
)

var (
	client   *eth.Web3Client
	contract *eth.Contract
	ipfs     *shell.Shell
	storage  *storagedb.Storage
)

func must(err error) {
	if err != nil {
		log.Panicln(err)
	}
}

func load() error {
	
	if client != nil {
		// already initialized
		return nil
	}

	var err error

	if err = loadEthClient() ; err != nil {
		return err
	}
	if err = loadIPFSClient() ; err != nil {
		return err
	}
	if err = loadContract() ; err != nil {
		return err
	}
	if err = loadStorage() ; err != nil {
		return err
	}
	return nil
}

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

func loadStorage() error {

	var err error

	storage,err = storagedb.New()

	return err

}

func loadEthClient() error {

	var err error

	ks := keystore.NewKeyStore(cfg.C.Keystore.Path, keystore.StandardScryptN, keystore.StandardScryptP)
	account, err := ks.Find(accounts.Account{
		Address: common.HexToAddress(cfg.C.Keystore.Account),
	})
	if err != nil {
		return err
	}

	err = ks.Unlock(account, cfg.C.Keystore.Passwd)
	if err != nil {
		return err
	}

	client, err = eth.NewWeb3Client(
		cfg.C.Web3.RPCURL,
		ks,
		account,
	)
	if err != nil {
		return err
	}

	client.ClientMutex = &sync.Mutex{}

	accountInfo, err := client.AccountInfo()
	if err != nil {
		return err
	}

	log.Println("Account info: ", accountInfo)
	return nil
}

func loadContract() error {

	var err error

	log.Println("Downloading ", cfg.C.Contracts.IPFSProxy.JSONURL)
	resp, err := http.Get(cfg.C.Contracts.IPFSProxy.JSONURL)
	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if cfg.C.Contracts.IPFSProxy.Address != "" {
		address := common.HexToAddress(cfg.C.Contracts.IPFSProxy.Address)
		contract, err = eth.NewContractFromJson(client, resp.Body, &address)
	} else {
		contract, err = eth.NewContractFromJson(client, resp.Body, nil)
	}
	return err

}

func loadIPFSClient() error {
	ipfs = shell.NewLocalShell()
	if !ipfs.IsUp() {
		return fmt.Errorf("Cannot connect with local IPFS node")
	}
	return nil
}


