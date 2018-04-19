package cmd

import (
	"math/big"
	"time"

	"github.com/spf13/cobra"

	eth "github.com/adriamb/goic/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/sirupsen/logrus"
)

var (
	eventDispatcher *eth.EventDispatcher
)

func handleHashAdded(eventlog *types.Log) error {

	type HashAdded struct {
		Hash   string
		Ttl    *big.Int
	}

	var event HashAdded
	err := contract.Abi().Unpack(&event, "HashAdded", eventlog.Data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
		"TTL": event.Ttl.String(),
	  }).Debug("EVENT HashAdded");

	// get the size & pin
	stats, err := ipfs.ObjectStat(event.Hash)
	if err != nil {
		return err
	}	
	log.WithFields(log.Fields{
		"Hash": event.Hash,
		"DataSize": stats.DataSize,
	  }).Debug("IPFS stats");

	err = ipfs.Pin(event.Hash)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
	  }).Debug("IPFS pinning ok");

	// store in the DB
	err = storage.AddHash(
		eventlog.Address.Hex(),event.Hash,
		uint(event.Ttl.Int64()),uint(stats.DataSize),
	)

	if err != nil {
		return err
	}
	
	return nil
}

func handleHashRemoved(eventlog *types.Log) error {

	type HashRemoved struct {
		Hash   string
	}

	var event HashRemoved
	err := contract.Abi().Unpack(&event, "HashRemoved", eventlog.Data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
	  }).Debug("EVENT HashRemoved");

	// unpin
	err = ipfs.Unpin(event.Hash)
	if err != nil {
		log.Println("IPFS says that hash ",event.Hash," is not pinned.")
	}

	// remove from DB
	err = storage.RemoveHash(eventlog.Address.Hex(),event.Hash)
	if err != nil {
		return err
	}

	return nil
}

func addContract(address common.Address, ttl uint64) {

	abi := contract.Abi()

	eventDispatcher.MustRegister(address, abi, "HashAdded", handleHashAdded)
	eventDispatcher.MustRegister(address, abi, "HashRemoved", handleHashRemoved)

}

func handleContractAdded(eventlog *types.Log) error {

	type ContractAdded struct {
		Member common.Address
		PubKey common.Address
		Ttl    *big.Int
	}

	var event ContractAdded
	err := contract.Abi().Unpack(&event, "ContractAdded", eventlog.Data)
	if err != nil {
		return err
	}

	addContract(event.PubKey, event.Ttl.Uint64())

	return nil
}

func cmdServe(cmd *cobra.Command, args []string) {

	must(load())

	log.Println("goic address is ", contract.Address().Hex())

	// -- register handlers for main function

	eventDispatcher = eth.NewEventDispatcher(client.Client)

	address, abi := *contract.Address(), contract.Abi()

	eventDispatcher.MustRegister(address, abi, "ContractAdded", handleContractAdded)
	//eventDispatcher.MustRegister(address, abi, "ContractRemoved", handleContractRemoved)
	//eventDispatcher.MustRegister(address, abi, "PersistLimitChanged", handlePersistLimitChanged)

	addContract(address, 0)
	
	eventDispatcher.Start()

	cmdAddHash(nil,[]string{"QmS4ustL54uo8FzR9455qaxZwuMiUhyvMcX9Ba8nUH4uVv","1000"})

	<-time.After(time.Second * 3600)
}
