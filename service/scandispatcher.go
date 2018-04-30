package service

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	abi "github.com/ethereum/go-ethereum/accounts/abi"
	common "github.com/ethereum/go-ethereum/common"
	types "github.com/ethereum/go-ethereum/core/types"
	ethclient "github.com/ethereum/go-ethereum/ethclient"

	log "github.com/sirupsen/logrus"
)

type EventHandlerFunc func(*types.Log) error

type scanEventHandler struct {
	Address        common.Address
	EventSignature string
	Topic          string
	Handler        EventHandlerFunc
}

type ScanEventDispatcher struct {
	sync.Mutex
	client *ethclient.Client

	eventHandlers []scanEventHandler

	savepoint *SavePoint

	block    *types.Block
	receipts *ReceiptDownloader

	terminatech  chan interface{}
	terminatedch chan interface{}

	nextBlock    uint64
	nextTxIndex  uint
	nextLogIndex uint
}

func NewScanEventDispatcher(client *ethclient.Client, savepoint *SavePoint) *ScanEventDispatcher {

	return &ScanEventDispatcher{
		client:    client,
		savepoint: savepoint,

		receipts: NewReceiptDownloader(client, 10),

		terminatech:  make(chan interface{}),
		terminatedch: make(chan interface{}),
	}
}

// Register registers a function to be called on event emission. Call NotifyUpdate if this is done
//   when everything started
func (e *ScanEventDispatcher) RegisterHandler(address common.Address, abi *abi.ABI, event string, handler EventHandlerFunc) {

	abievent, ok := abi.Events[event]
	if !ok {
		panic(fmt.Errorf("Event %v not found", event))
	}
	topicID := abievent.Id()

	eventHandler := scanEventHandler{
		Address:        address,
		EventSignature: abievent.String(),
		Topic:          "0x" + hex.EncodeToString(topicID[:]),
		Handler:        handler,
	}

	e.Lock()
	e.eventHandlers = append(e.eventHandlers, eventHandler)
	e.Unlock()
}

func (e *ScanEventDispatcher) runHandlerFor(logevent *types.Log) error {

	if !logevent.Removed {

		var handler *scanEventHandler
		e.Lock()
		for _, v := range e.eventHandlers {
			if logevent.Address == v.Address && logevent.Topics[0].Hex() == v.Topic {
				handler = &v
				break
			}
		}
		e.Unlock()

		if handler != nil {
			log.WithField("event", handler.EventSignature).Debug("EVENT run handler ")
			return handler.Handler(logevent)
		}
	}

	return nil
}

func (e *ScanEventDispatcher) process() (bool, error) {

	var err error

	// Retrieve the last processed log, this is only called in the first loop
	if e.nextBlock == 0 {
		e.nextBlock, e.nextTxIndex, e.nextLogIndex, err = e.savepoint.Load()
		if err != nil {
			return false, err
		}
		e.nextLogIndex++
	}

	log.WithFields(log.Fields{
		"block":    e.nextBlock,
		"txindex":  e.nextTxIndex,
		"logindex": e.nextLogIndex,
	}).Debug("EVENT scanning blockchain")

	// Check if e.block is valid, if not download it
	if e.block == nil || e.block.NumberU64() < e.nextBlock {
		e.block, err = e.client.BlockByNumber(context.TODO(), big.NewInt(int64(e.nextBlock)))
		if err == ethereum.NotFound {
			// still not available
			return true, nil
		}
		if err != nil {
			return false, err
		}
		for index := e.nextTxIndex; index < uint(len(e.block.Transactions())); index++ {
			log.Debug("Enqueueing ", e.block.Transactions()[index].Hash().Hex())
			e.receipts.Request(e.block.Transactions()[index].Hash())
		}
	}

	// Download the receipt that contains the log
	var receipt *types.Receipt
	log.Debug("e.nextTxIndex=", e.nextTxIndex, " len(e.block.Transactions())=", len(e.block.Transactions()))
	if e.nextTxIndex < uint(len(e.block.Transactions())) {

		txid := e.block.Transactions()[e.nextTxIndex].Hash()
		log.Debug("Reading receipt ", txid.Hex())

		receipt, err := e.receipts.Get(txid)
		if err != nil {
			return false, err
		}

		log.Debug("Readed receipt ", txid.Hex())

		// Is marked the transaction to be skipped?
		skip, err := e.savepoint.SkipTx(txid)
		if err != nil {
			return false, err
		}

		if skip {
			e.receipts.Forget(receipt.TxHash)
		}
	}

	if receipt != nil && e.nextLogIndex < uint(len(receipt.Logs)) {
		logevent := receipt.Logs[e.nextLogIndex]
		err = e.runHandlerFor(logevent)
		if err != nil {
			log.WithFields(log.Fields{
				"err":  err,
				"txid": receipt.TxHash.Hex(),
			}).Warn("EVENT Failed handling ")
			return false, err
		}

		e.savepoint.Save(logevent)
		e.nextLogIndex++
	}

	/* go to next */

	if receipt == nil || e.nextLogIndex >= uint(len(receipt.Logs)) {
		if receipt != nil {
			e.receipts.Forget(receipt.TxHash)
		}
		e.nextLogIndex = 0
		e.nextTxIndex++
	}

	if e.nextTxIndex >= uint(len(e.block.Transactions())) {
		e.nextLogIndex = 0
		e.nextTxIndex = 0
		e.nextBlock++
	}

	return false, nil
}

func (e *ScanEventDispatcher) QueryStop() {
	go func() {
		e.terminatech <- nil
	}()
}

func (e *ScanEventDispatcher) WaitStopped() {
	<-e.terminatedch
}

func (e *ScanEventDispatcher) Start() {

	go func() {
		e.receipts.Start()
		for true {
			select {

			case <-e.terminatech:
				log.Debug("EVENT Dispatching terminatech")
				e.terminatedch <- nil
				e.receipts.QueryStop()
				e.receipts.WaitStopped()
				return

			default:
				wait, err := e.process()
				if err != nil {
					go func() {
						log.Error("EVENT Failed ", err)
						e.terminatech <- nil
					}()
				} else if wait {
					time.Sleep(4 * time.Second)
				}
			}
		}

	}()
}
