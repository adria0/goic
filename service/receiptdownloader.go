package service

import (
	"bytes"
	"context"
	"errors"
	"math/rand"
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

const (
	retryDownloadTimes = 5
)

type receiptTask struct {
	Tx      common.Hash
	Receipt *types.Receipt
	err     chan error
}

type ReceiptDownloader struct {
	sync.Mutex

	client *ethclient.Client

	concurrency int
	queue       []*receiptTask
	pending     map[common.Hash]*receiptTask

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

func (r *ReceiptDownloader) Request(txid common.Hash) {
	r.Lock()
	r.queue = append(r.queue, &receiptTask{
		Tx:  txid,
		err: make(chan error),
	})
	r.Unlock()

	r.next()
}

func (r *ReceiptDownloader) Get(txid common.Hash) (*types.Receipt, error) {

	r.Lock()
	task, ok := r.pending[txid]
	if !ok {
		// Look at the queue, this is supposed that not happens often.
		for _, v := range r.queue {
			if bytes.Compare(txid[:], v.Tx[:]) == 0 {
				task = v
				ok = true
				break
			}
		}
	}
	r.Unlock()

	if !ok {
		log.WithField("tx", txid.Hex()).Warn(errTxNotFound)
		return nil, errTxNotFound
	}

	err := <-task.err

	return task.Receipt, err
}

func (r *ReceiptDownloader) Forget(txid common.Hash) {
	r.Lock()
	if _, exists := r.pending[txid]; exists {
		delete(r.pending, txid)
	} else {
		log.WithField("txid", txid.Hex()).Warn("RDOWN cannot forget tx")
	}
	r.Unlock()
}

func (r *ReceiptDownloader) Stats() (queuelen, pendinglen int) {
	r.Lock()
	queuelen = len(r.queue)
	pendinglen = len(r.pending)
	r.Unlock()

	return
}

func (r *ReceiptDownloader) next() {

	r.Lock()

	if len(r.queue) == 0 || len(r.pending) >= r.concurrency {
		r.Unlock()
		return
	}

	task := r.queue[0]
	r.queue = r.queue[1:]

	r.pending[task.Tx] = task

	r.Unlock()

	go func() {

		log.WithField("txid", task.Tx.Hex()).Debug("RDOWN Downloading receipt")

		err := errors.New("")
		var receipt *types.Receipt

		for retry := retryDownloadTimes; err != nil && retry > 0; retry-- {

			receipt, err = r.client.TransactionReceipt(context.TODO(), task.Tx)
			if err != nil {
				log.WithFields(log.Fields{
					"txid":  task.Tx.Hex(),
					"retry": retry,
					"err":   err,
				}).Warn("RDOWN failed to download receipt")

				<-time.After(time.Duration(rand.Int63n(int64(time.Second))))
			}
		}

		r.Lock()
		task := r.pending[task.Tx]
		r.Unlock()

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
