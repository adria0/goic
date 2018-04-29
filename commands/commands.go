package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	cfg "github.com/adriamb/goic/config"
	"github.com/adriamb/goic/service"
	sto "github.com/adriamb/goic/storage"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func AddHash(cmd *cobra.Command, args []string) {

	if len(args) != 2 {
		must(fmt.Errorf("addhash <ipfshash> <ttl>"))
	}
	must(load(false))

	ttl := new(big.Int)
	ttl.SetString(args[1], 10)
	_, _, err := contract.SendTransactionSync(
		big.NewInt(0), 0,
		"addHash", args[0], ttl,
	)
	must(err)
}

func RemoveHash(cmd *cobra.Command, args []string) {
	if len(args) != 1 {
		must(fmt.Errorf("rmhash <ipfshash>"))
	}
	must(load(false))
	_, _, err := contract.SendTransactionSync(
		big.NewInt(0), 0,
		"removeHash", args[0],
	)
	must(err)
}

func AddSkipTx(cmd *cobra.Command, args []string) {

	if len(args) != 1 {
		must(fmt.Errorf("skiptx <txhash>"))
	}
	must(load(true))
	must(storage.AddSkipTx(common.HexToHash(args[0])))
}

func Deploy(cmd *cobra.Command, args []string) {

	must(load(true))

	// -- deploy contract

	members := make([]common.Address, len(cfg.C.Contracts.IPFSProxy.Deploy.Members))
	for i, v := range cfg.C.Contracts.IPFSProxy.Deploy.Members {
		members[i] = common.HexToAddress(v)
	}

	_, _, err := contract.DeploySync(
		members,
		big.NewInt(int64(cfg.C.Contracts.IPFSProxy.Deploy.Required)),
		big.NewInt(int64(cfg.C.Contracts.IPFSProxy.Deploy.PersistLimit)),
	)
	must(err)
	log.WithField("address", contract.Address().Hex()).Info("Goic contract deployed")

	err = storage.AddContract(*contract.Address())
	must(err)

}

func Config(cmd *cobra.Command, args []string) {

	json, _ := json.MarshalIndent(cfg.C, "", "  ")
	log.Println("Efective configuration: " + string(json))

}

func DumpDb(cmd *cobra.Command, args []string) {

	must(loadStorage())

	storage.Dump(os.Stdout)
}

func InitDb(cmd *cobra.Command, args []string) {

	must(load(true))

	storage.SetGlobals(sto.GlobalsEntry{
		CurrentQuota: 0,
		LastBlock:    0,
		LastLogIndex: 0,
		LastTxIndex:  0,
	})
}

func Test(cmd *cobra.Command, args []string) {
	client, err := ethclient.Dial("http://localhost:8545")
	must(err)
	download := service.NewReceiptDownloader(client, 10)
	download.Start()

	block := 5527294
	for blockno := block - 2; blockno < block; blockno++ {
		block, err := client.BlockByNumber(context.TODO(), big.NewInt(int64(blockno)))
		must(err)
		for index, tx := range block.Transactions() {
			download.Request(uint(index), tx.Hash())
		}
		for _, tx := range block.Transactions() {
			receipt, err := download.Get(tx.Hash())
			download.Forget(tx.Hash())
			must(err)
			log.Info("count====>", len(receipt.Logs))
		}
	}
	download.QueryStop()
	download.WaitStopped()

}

func Serve(cmd *cobra.Command, args []string) {

	must(load(true))

	service := service.NewService(
		client, contract, ipfs, storage,
	)

	service.Serve()

}
