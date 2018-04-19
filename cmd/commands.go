package cmd

import (
	"math/big"
	"fmt"
	"encoding/json"
	"github.com/spf13/cobra"
	cfg "github.com/adriamb/goic/config"
	"github.com/ethereum/go-ethereum/common"
	log "github.com/sirupsen/logrus"	
)

func cmdAddHash(cmd *cobra.Command, args []string) {

	if len(args)!=2 {
		must(fmt.Errorf("addhash <ipfshash> <ttl>"))
	}
	must(load())

	ttl := new(big.Int)
	ttl.SetString(args[1],10)
	tx, _, err := contract.SendTransactionSync(
		big.NewInt(0), 0,
		"addHash",args[0],ttl,
	)
	must(err)
	log.Info("tx ",tx.Hash().Hex())
}

func cmdRemoveHash(cmd *cobra.Command, args []string)  {
	if len(args)!=1 {
		must(fmt.Errorf("rmhash <ipfshash>"))
	}
	must(load())
	tx, _, err := contract.SendTransactionSync(
		big.NewInt(0), 0,
		 "removeHash",args[0],
	)
	must(err)
	log.Info("tx ",tx.Hash().Hex())
}

func cmdDeploy(cmd *cobra.Command, args []string){

	must(load())

	// -- deploy contract

	members := make([]common.Address,len(cfg.C.Contracts.IPFSProxy.Deploy.Members))
	for i,v := range cfg.C.Contracts.IPFSProxy.Deploy.Members {
		members[i] = common.HexToAddress(v)
	}
	
	_, _, err := contract.DeploySync(
		members,
		big.NewInt(int64(cfg.C.Contracts.IPFSProxy.Deploy.Required)),
		big.NewInt(int64(cfg.C.Contracts.IPFSProxy.Deploy.PersistLimit)),
	)
	must(err)
	log.Info("goic contract deployed at ", contract.Address().Hex())

}

func cmdConfig(cmd *cobra.Command, args []string){

	json, _ := json.MarshalIndent(cfg.C, "", "  ")
	log.Println("Efective configuration: " + string(json))


}