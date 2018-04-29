package service

import (
	"errors"
	"math/big"
	"os"
	"os/signal"

	eth "github.com/adriamb/goic/eth"
	sto "github.com/adriamb/goic/storage"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	shell "github.com/ipfs/go-ipfs-api"
	log "github.com/sirupsen/logrus"
)

type Service struct {
	eventDispatcher *ScanEventDispatcher
	persistLimit    *big.Int
	client          *eth.Web3Client
	contract        *eth.Contract
	ipfs            *shell.Shell
	storage         *sto.Storage
}

func NewService(client *eth.Web3Client, contract *eth.Contract, ipfs *shell.Shell, storage *sto.Storage) *Service {
	return &Service{
		eventDispatcher: NewScanEventDispatcher(client.Client, NewSavePoint(storage)),
		client:          client,
		contract:        contract,
		ipfs:            ipfs,
		storage:         storage,
	}
}

func (s *Service) handleHashAdded(eventlog *types.Log) error {

	type HashAdded struct {
		Hash string
		Ttl  *big.Int
	}

	var event HashAdded
	err := s.contract.Abi().Unpack(&event, "HashAdded", eventlog.Data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
		"TTL":  event.Ttl.String(),
	}).Info("SERVE HashAdded")

	// get the size & pin
	stats, err := s.ipfs.ObjectStat(event.Hash)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash":     event.Hash,
		"DataSize": stats.DataSize,
	}).Debug("IPFS stats")

	err = s.ipfs.Pin(event.Hash)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
	}).Debug("IPFS pinning ok")

	// store in the DB
	err = s.storage.AddHash(
		eventlog.Address, event.Hash,
		uint(event.Ttl.Int64()), uint(stats.DataSize),
	)

	if err != nil {
		return err
	}

	return nil
}

func (s *Service) handleHashRemoved(eventlog *types.Log) error {

	type HashRemoved struct {
		Hash string
	}

	var event HashRemoved
	err := s.contract.Abi().Unpack(&event, "HashRemoved", eventlog.Data)
	if err != nil {
		return err
	}

	log.WithFields(log.Fields{
		"Hash": event.Hash,
	}).Info("SERVE HashRemoved")

	// remove from DB
	isLastHash, err := s.storage.RemoveHash(eventlog.Address, event.Hash)
	if err != nil {
		return err
	}

	if isLastHash {
		// is the last has registerer in a contract, unpin it
		err = s.ipfs.Unpin(event.Hash)
		if err != nil {
			log.Warn("IPFS says that hash ", event.Hash, " is not pinned.")
		}
	}

	return nil
}

func (s *Service) registerContractHandlers(address common.Address) {

	log.WithField("contract", address.Hex()).Info("SERVE start listening hashAdd/hashRemove")

	abi := s.contract.Abi()

	s.eventDispatcher.RegisterHandler(address, abi, "HashAdded", s.handleHashAdded)
	s.eventDispatcher.RegisterHandler(address, abi, "HashRemoved", s.handleHashRemoved)
}

func (s *Service) handleContractAdded(eventlog *types.Log) error {

	type ContractAdded struct {
		Member common.Address
		PubKey common.Address
		Ttl    *big.Int
	}

	var event ContractAdded
	err := s.contract.Abi().Unpack(&event, "ContractAdded", eventlog.Data)
	if err != nil {
		return err
	}

	s.registerContractHandlers(event.PubKey)

	return nil
}

func (s *Service) Serve() error {

	var err error

	// -- read current persistlimit

	if err = s.contract.VerifyBytecode(); err != nil {
		return errors.New("cannot verify deployed smartcontract")
	}

	if err = s.contract.Call(&s.persistLimit, "persistLimit"); err != nil {
		return errors.New("error reading current persistLimit")
	}

	proxyAddress, abi := *s.contract.Address(), s.contract.Abi()

	s.eventDispatcher.RegisterHandler(proxyAddress, abi, "ContractAdded", s.handleContractAdded)

	var contracts []common.Address
	if contracts, err = s.storage.Contracts(); err != nil {
		return err
	}

	// update DB is proxy contract is not added as contract

	isProxyContractInDb := false
	for _, contract := range contracts {
		if contract == proxyAddress {
			isProxyContractInDb = true
		}
	}
	if !isProxyContractInDb {
		s.storage.AddContract(proxyAddress)
		contracts = append(contracts, proxyAddress)
	}

	// -- register handlers for addHash/addRemove

	for _, contract := range contracts {
		s.registerContractHandlers(contract)
	}

	//eventDispatcher.MustRegister(address, abi, "ContractRemoved", handleContractRemoved)
	//eventDispatcher.MustRegister(address, abi, "PersistLimitChanged", handlePersistLimitChanged)

	s.eventDispatcher.Start()

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		for _ = range c {
			log.Info("SERVE interrupt signal got. Finishing.")
			s.eventDispatcher.QueryStop()
			break
		}
	}()
	s.eventDispatcher.WaitStopped()

	return nil
}
