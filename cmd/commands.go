package cmd

import (
	"encoding/json"
	"fmt"
	"math/big"
	"os"

	cfg "github.com/adriamb/goic/config"
	sto "github.com/adriamb/goic/storage"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func cmdAddHash(cmd *cobra.Command, args []string) {

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

func cmdRemoveHash(cmd *cobra.Command, args []string) {
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

func cmdAddSkipTx(cmd *cobra.Command, args []string) {

	if len(args) != 1 {
		must(fmt.Errorf("skiptx <txhash>"))
	}
	must(load(true))
	must(storage.AddSkipTx(common.HexToHash(args[0])))
}

func cmdDeploy(cmd *cobra.Command, args []string) {

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

func cmdConfig(cmd *cobra.Command, args []string) {

	json, _ := json.MarshalIndent(cfg.C, "", "  ")
	log.Println("Efective configuration: " + string(json))

}

func cmdDumpDb(cmd *cobra.Command, args []string) {

	must(loadStorage())

	storage.Dump(os.Stdout)
}

func cmdInitDb(cmd *cobra.Command, args []string) {

	must(load(true))

	storage.SetGlobals(sto.GlobalsEntry{
		CurrentQuota: 0,
		LastBlock:    0,
		LastLogIndex: 0,
		LastTxIndex:  0,
	})
}
