package eth

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

type scanEventHandler struct {
	Address        common.Address
	EventSignature string
	Topic          string
	Handler        EventHandlerFunc
}

type ScanEventDispatcher struct {
	client        *ethclient.Client
	eventHandlers []scanEventHandler

	mutex *sync.Mutex

	savepoint SavePoint

	block *types.Block

	receipts map[common.Hash]*types.Receipt

	nextBlock    uint64
	nextTxIndex  uint
	nextLogIndex uint

	terminatech  chan bool
	terminatedch chan bool
}

func NewScanEventDispatcher(client *ethclient.Client, savepoint SavePoint) *ScanEventDispatcher {

	return &ScanEventDispatcher{
		client:    client,
		savepoint: savepoint,
		mutex:     &sync.Mutex{},

		terminatech:  make(chan bool),
		terminatedch: make(chan bool),
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

	e.mutex.Lock()
	e.eventHandlers = append(e.eventHandlers, eventHandler)
	e.mutex.Unlock()
}

// NotifyUpdate updates the registers
func (e *ScanEventDispatcher) NotifyHandlersChanged() {

}

func (e *ScanEventDispatcher) process() (bool, error) {

	var err error

	if e.nextBlock == 0 {
		/* retrieve the last processed transaction */
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

	// check if e.block is valid, if not download it
	if e.block == nil || e.block.NumberU64() < e.nextBlock {

		e.block, err = e.client.BlockByNumber(context.TODO(), big.NewInt(int64(e.nextBlock)))
		if err == ethereum.NotFound {
			return true, nil
		}
		if err != nil {
			return false, err
		}
		e.receipts = make(map[common.Hash]*types.Receipt)
	}

	var receipt *types.Receipt

	if e.nextTxIndex < uint(len(e.block.Transactions())) {

		txid := e.block.Transactions()[e.nextTxIndex].Hash()
		skip, err := e.savepoint.SkipTx(txid)
		if err != nil {
			return false, err
		}

		if !skip {
			var exists bool
			receipt, exists = e.receipts[txid]
			if !exists {
				receipt, err = e.client.TransactionReceipt(context.TODO(), txid)
				if err != nil {
					return false, err
				}
				e.receipts[txid] = receipt
			}
		}
	}

	if receipt != nil && e.nextLogIndex < uint(len(receipt.Logs)) {

		logevent := receipt.Logs[e.nextLogIndex]

		if !logevent.Removed {

			var handler *scanEventHandler
			e.mutex.Lock()
			for _, v := range e.eventHandlers {
				if logevent.Address == v.Address && logevent.Topics[0].Hex() == v.Topic {
					handler = &v
					break
				}
			}
			e.mutex.Unlock()

			if handler != nil {
				log.WithField("event", handler.EventSignature).Debug("EVENT run handler ")
				err := handler.Handler(logevent)
				if err != nil {
					log.WithFields(log.Fields{
						"err":  err,
						"txid": receipt.TxHash.Hex(),
					}).Warn("EVENT Failed handling ")
					return false, err
				}
			}
		}

		e.savepoint.Save(logevent)
		e.nextLogIndex++
	}

	/* go to next */

	if receipt == nil || e.nextLogIndex >= uint(len(receipt.Logs)) {
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
		e.terminatech <- true
	}()
}

func (e *ScanEventDispatcher) WaitStopped() {
	<-e.terminatedch
}

func (e *ScanEventDispatcher) Start() {

	go func() {
		for true {
			select {

			case <-e.terminatech:
				log.Debug("EVENT Dispatching terminatech")
				e.terminatedch <- true
				return

			default:
				wait, err := e.process()
				if err != nil {
					go func() {
						log.Error("EVENT Failed ", err)
						e.terminatech <- true
					}()
				} else if wait {
					time.Sleep(4 * time.Second)
				}
			}
		}

	}()
}
