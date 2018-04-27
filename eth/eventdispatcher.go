package eth

import (
	abi "github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

type EventHandlerFunc func(*types.Log) error

// Notifies the last block, transaction and event has been processed
type SavePoint interface {
	Load() (uint64, uint, uint, error)
	SkipTx(common.Hash) (bool, error)
	Save(*types.Log) error
}

type EventDispatcher interface {
	RegisterHandler(address common.Address, abi *abi.ABI, event string, handler EventHandlerFunc)
	NotifyHandlersChanged()
	Start()
	QueryStop()
	WaitStopped()
}
