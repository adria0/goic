package cmd

import (
	"log"
	"math/big"
	"time"

	eth "github.com/adriamb/goic/eth"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type Member struct {
	address common.Address
	ttl     uint64
	files   map[string]uint64 // hash -> ttl
}

var (
	members         map[common.Address]Member
	eventDispatcher *eth.EventDispatcher
)

func handleHashAdded(eventlog *types.Log) error {
	//	event HashAdded(address pubKey, string hashAdded, uint ttl);

	type HashAdded struct {
		PubKey common.Address
		Hash   string
		TTL    *big.Int
	}

	var event HashAdded
	err := contract.Abi().Unpack(&event, "HashAdded", eventlog.Data)
	if err != nil {
		return err
	}

	members[eventlog.Address].files[event.Hash] = event.TTL.Uint64()

	return nil
}

func handleHashRemoved(eventlog *types.Log) error {
	//	event HashRemoved(address pubKey, string hashRemoved);

	type HashRemoved struct {
		PubKey common.Address
		Hash   string
	}

	var event HashRemoved
	err := contract.Abi().Unpack(&event, "HashRemoved", eventlog.Data)
	if err != nil {
		return err
	}

	delete(members[eventlog.Address].files, event.Hash)

	return nil
}

func addContract(address common.Address, ttl uint64) {

	abi := contract.Abi()

	eventDispatcher.MustRegister(address, abi, "HashAdded", handleHashAdded)
	eventDispatcher.MustRegister(address, abi, "HashAdded", handleHashRemoved)

	members[address] = Member{
		address: address,
		ttl:     ttl,
		files:   make(map[string]uint64),
	}

}

func handleContractAdded(eventlog *types.Log) error {

	type ContractAdded struct {
		Member common.Address
		PubKey common.Address
		TTL    *big.Int
	}

	var event ContractAdded
	err := contract.Abi().Unpack(&event, "ContractAdded", eventlog.Data)
	if err != nil {
		return err
	}

	addContract(event.PubKey, event.TTL.Uint64())

	return nil
}

func mustServe() {

	mustLoad()

	log.Println("goic address is ", contract.Address().Hex())

	// -- register handlers for main function

	members = make(map[common.Address]Member)

	eventDispatcher = eth.NewEventDispatcher(client.Client)

	address, abi := *contract.Address(), contract.Abi()

	eventDispatcher.MustRegister(address, abi, "ContractAdded", handleContractAdded)
	//eventDispatcher.MustRegister(address, abi, "ContractRemoved", handleContractRemoved)
	//eventDispatcher.MustRegister(address, abi, "PersistLimitChanged", handlePersistLimitChanged)

	addContract(address, 0)

	eventDispatcher.Start()

	<-time.After(time.Second * 3600)
}
