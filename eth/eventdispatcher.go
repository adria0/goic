package eth

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"

	ethereum "github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

type EventHandlerFunc func(*types.Log) error

// EventHandler associates a function to an event
type EventHandler struct {
	Address        common.Address
	EventSignature string
	Topic          string
	Handler        EventHandlerFunc
}

type EventDispatcher struct {
	client *ethclient.Client

	mutex *sync.Mutex
	queue []*types.Log

	filterch     chan types.Log
	nextch       chan bool
	installch    chan bool
	terminatech  chan bool
	terminatedch chan bool

	subscription  ethereum.Subscription
	eventHandlers []EventHandler
}

func NewEventDispatcher(client *ethclient.Client) *EventDispatcher {

	return &EventDispatcher{
		client: client,
		mutex:  &sync.Mutex{},
		queue:  []*types.Log{},

		nextch:       make(chan bool),
		installch:    make(chan bool),
		terminatech:  make(chan bool),
		terminatedch: make(chan bool),
	}
}

// Register registers a function to be called on event emission. Call NotifyUpdate if this is done
//   when everything started
func (e *EventDispatcher) MustRegister(address common.Address, abi *abi.ABI, event string, handler EventHandlerFunc) {

	abievent, ok := abi.Events[event]
	if !ok {
		panic(fmt.Errorf("Event %v not found", event))
	}
	topicID := abievent.Id()

	eventHandler := EventHandler{
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
func (e *EventDispatcher) NotifyUpdate() {
	e.installch <- true
}

func (e *EventDispatcher) append(eventlog *types.Log) {
	e.mutex.Lock()
	e.queue = append(e.queue, eventlog)
	e.mutex.Unlock()
}

func (e *EventDispatcher) process() {

	defer func() {
		e.nextch <- true
	}()

	e.mutex.Lock()
	var logevent *types.Log
	if len(e.queue) > 0 {
		logevent = e.queue[0]
		e.queue = e.queue[1:]
	}
	e.mutex.Unlock()

	if logevent == nil {
		time.Sleep(200)
		return
	}

	if logevent.Removed {
		return
	}

	var handler EventHandlerFunc

	e.mutex.Lock()
	for _, v := range e.eventHandlers {
		if logevent.Address == v.Address && logevent.Topics[0].Hex() == v.Topic {
			handler = v.Handler
			break
		}
	}
	e.mutex.Unlock()

	if handler != nil {
		handler(logevent)
	}

}

func (e *EventDispatcher) QueryStopAndWait() {
	e.terminatech <- true
	<-e.terminatedch
}

func (e *EventDispatcher) addresses() []common.Address {

	addrs := []common.Address{}

	for _, v := range e.eventHandlers {
		found := false
		for _, addr := range addrs {
			if addr == v.Address {
				found = true
				break
			}
		}
		if !found {
			addrs = append(addrs, v.Address)
		}
	}

	return addrs
}

func (e *EventDispatcher) install() {

	ctx := context.TODO()
	filterch := make(chan types.Log)

	addrs := e.addresses()

	query := ethereum.FilterQuery{
		FromBlock: nil,
		ToBlock:   nil,
		Addresses: addrs,
		Topics:    [][]common.Hash{{}}, // optimize
	}

	subscripton, err := e.client.SubscribeFilterLogs(ctx, query, filterch)
	if err != nil {
		// TODO log something
		return
	}

	if e.filterch != nil {
		for len(e.filterch) > 0 {
			eventlog := <-e.filterch
			// TODO ??? this is going to work?
			e.append(&eventlog)
		}
		e.subscription.Unsubscribe()
	}

	e.subscription = subscripton
	e.filterch = filterch
}

func (e *EventDispatcher) Start() {

	go func() {
		for true {
			select {

			case <-e.terminatech:
				break

			case <-e.installch:
				e.install()

			case logevent := <-e.filterch:
				e.append(&logevent)

			case <-e.nextch:
				e.process()

			}
		}
		e.terminatedch <- true
	}()

	e.installch <- true
	e.nextch <- true
}

func debugLog(eventlog *types.Log) {
	log.Println("Log from address", eventlog.Address.Hex())
	for c, t := range eventlog.Topics {
		log.Printf("  Topic[%v]: %v", c, t.Hex())
	}
	log.Println("  Data:", hex.EncodeToString(eventlog.Data))
}
