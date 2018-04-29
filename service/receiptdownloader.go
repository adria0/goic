package service

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	log "github.com/sirupsen/logrus"
)

var (
	errTxNotFound = errors.New("tx not found")
)

type receiptTask struct {
	Tx      common.Hash
	Receipt *types.Receipt
	err     chan error
}

type ReceiptDownloader struct {
	sync.Mutex

	concurrency int

	client *ethclient.Client
	queue  []*receiptTask

	pending map[common.Hash]*receiptTask

	nextch       chan interface{}
	terminatech  chan interface{}
	terminatedch chan interface{}
}

func NewReceiptDownloader(client *ethclient.Client, concurrency int) *ReceiptDownloader {

	return &ReceiptDownloader{
		concurrency:  concurrency,
		client:       client,
		queue:        []*receiptTask{},
		pending:      make(map[common.Hash]*receiptTask),
		nextch:       make(chan interface{}, concurrency),
		terminatech:  make(chan interface{}),
		terminatedch: make(chan interface{}),
	}

}

func (r *ReceiptDownloader) Request(index uint, txid common.Hash) {
	r.Lock()
	r.queue = append(r.queue, &receiptTask{
		Tx: txid,
	})
	r.Unlock()

	r.next()
}

func (r *ReceiptDownloader) Get(txid common.Hash) (*types.Receipt, error) {

	r.Lock()
	task, ok := r.pending[txid]
	r.Unlock()

	if !ok {
		return nil, errTxNotFound
	}

	err := <-task.err

	return task.Receipt, err
}

func (r *ReceiptDownloader) Forget(txid common.Hash) {
	r.Lock()
	if _, exists := r.pending[txid]; exists {
		delete(r.pending, txid)
	}
	r.Unlock()
}

func (r *ReceiptDownloader) next() {

	r.Lock()

	if len(r.queue) == 0 || len(r.pending) >= r.concurrency {
		r.Unlock()
		return
	}

	task := r.queue[0]
	r.queue = r.queue[1:]

	task.err = make(chan error)
	r.pending[task.Tx] = task

	r.Unlock()

	go func() {

		log.WithField("txid", task.Tx.Hex()).Debug("RDOWN Downloading receipt")
		receipt, err := r.client.TransactionReceipt(context.TODO(), task.Tx)
		log.WithFields(log.Fields{
			"txid": task.Tx.Hex(),
			"err":  err,
		}).Debug("RDOWN Downloaded receipt")

		r.Lock()
		task := r.pending[task.Tx]
		r.Unlock()

		log.Debug("RDOWN SETTING channel ", task.err)
		task.Receipt = receipt
		task.err <- err

		r.nextch <- nil

	}()

	r.next()
}

func (r *ReceiptDownloader) QueryStop() {
	go func() {
		r.terminatech <- nil
	}()
}

func (r *ReceiptDownloader) WaitStopped() {
	<-r.terminatedch
}

func (r *ReceiptDownloader) Start() {

	go func() {
		for true {
			select {
			case <-r.terminatech:
				log.Debug("RDOWN terminatech")
				r.terminatedch <- nil
				return

			case <-r.nextch:
				r.next()

			case <-time.After(4 * time.Second):
				log.Debug("RDOWN waiting")

			}
		}
		r.terminatedch <- nil
	}()
}
