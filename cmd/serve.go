package cmd

import (
	"math/big"
	"os"
	"os/signal"

	"github.com/spf13/cobra"

	eth "github.com/adriamb/goic/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	log "github.com/sirupsen/logrus"
)

var (
	eventDispatcher *eth.ScanEventDispatcher
	persistLimit    *big.Int
)

type savePoint struct {
}

func (savePoint) Load() (lastBlock uint64, lastTxIndex, lastLogIndex uint, err error) {
	globals, err := storage.Globals()
	if err != nil {
		return 0, 0, 0, err
	}
	return globals.LastBlock, globals.LastTxIndex, globals.LastLogIndex, nil
}

func (savePoint) Save(logevent *types.Log) error {
	globals, err := storage.Globals()
	if err != nil {
		return err
	}

	globals.LastBlock = logevent.BlockNumber
	globals.LastTxIndex = logevent.TxIndex
	globals.LastLogIndex = logevent.Index

	log.WithFields(log.Fields{
		"block":  globals.LastBlock,
		"txidx":  globals.LastTxIndex,
		"logidx": globals.LastLogIndex,
	}).Debug("SAVEP save")

	return storage.SetGlobals(*globals)
}

func (savePoint) SkipTx(txid common.Hash) (bool, error) {

	return storage.SkipTx(txid)
}

func handleHashAdded(eventlog *types.Log) error {

	type HashAdded struct {
		Hash string
		Ttl  *big.Int
	}

	var event HashAdded
	err := contract.Abi().Unpack(&event, "HashAdded", eventlog.Data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
		"TTL":  event.Ttl.String(),
	}).Info("SERVE HashAdded")

	// get the size & pin
	stats, err := ipfs.ObjectStat(event.Hash)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash":     event.Hash,
		"DataSize": stats.DataSize,
	}).Debug("IPFS stats")

	err = ipfs.Pin(event.Hash)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
	}).Debug("IPFS pinning ok")

	// store in the DB
	err = storage.AddHash(
		eventlog.Address, event.Hash,
		uint(event.Ttl.Int64()), uint(stats.DataSize),
	)

	if err != nil {
		return err
	}

	return nil
}

func handleHashRemoved(eventlog *types.Log) error {

	type HashRemoved struct {
		Hash string
	}

	var event HashRemoved
	err := contract.Abi().Unpack(&event, "HashRemoved", eventlog.Data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
	}).Info("SERVE HashRemoved")

	// remove from DB
	isLastHash, err := storage.RemoveHash(eventlog.Address, event.Hash)
	if err != nil {
		return err
	}

	if isLastHash {
		// is the last has registerer in a contract, unpin it
		err = ipfs.Unpin(event.Hash)
		if err != nil {
			log.Warn("IPFS says that hash ", event.Hash, " is not pinned.")
		}
	}

	return nil
}

func registerContractHandlers(address common.Address) {

	log.WithField("contract", address.Hex()).Info("SERVE start listening hashAdd/hashRemove")

	abi := contract.Abi()

	eventDispatcher.RegisterHandler(address, abi, "HashAdded", handleHashAdded)
	eventDispatcher.RegisterHandler(address, abi, "HashRemoved", handleHashRemoved)
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

	registerContractHandlers(event.PubKey)

	return nil
}

func cmdServe(cmd *cobra.Command, args []string) {

	must(load(true))

	// -- read current persistlimit

	must(contract.Call(&persistLimit, "persistLimit"))
	log.WithField("limit", persistLimit.Uint64()).Info("SERVE readed persistLimit from contract ")

	eventDispatcher = eth.NewScanEventDispatcher(client.Client, savePoint{})

	proxyAddress, abi := *contract.Address(), contract.Abi()

	eventDispatcher.RegisterHandler(proxyAddress, abi, "ContractAdded", handleContractAdded)
	contracts, err := storage.Contracts()
	must(err)

	// update DB is proxy contract is not added as contract

	isProxyContractInDb := false
	for _, contract := range contracts {
		if contract == proxyAddress {
			isProxyContractInDb = true
		}
	}
	if !isProxyContractInDb {
		storage.AddContract(proxyAddress)
		contracts = append(contracts, proxyAddress)
	}

	// -- register handlers for addHash/addRemove

	for _, contract := range contracts {
		registerContractHandlers(contract)
	}

	//eventDispatcher.MustRegister(address, abi, "ContractRemoved", handleContractRemoved)
	//eventDispatcher.MustRegister(address, abi, "PersistLimitChanged", handlePersistLimitChanged)

	eventDispatcher.Start()

	cmdAddHash(nil, []string{"QmYyWGsdKiXujTKLv7efg5HMSAX5VSnKpZaUdy7aRyoPh2", "1000"})

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			log.Info("SERVE interrupt signal got. Finishing.")
			eventDispatcher.QueryStop()
			break
		}
	}()
	eventDispatcher.WaitStopped()
}
