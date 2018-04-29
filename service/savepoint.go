package service

import (
	"github.com/adriamb/goic/storage"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"

	log "github.com/sirupsen/logrus"
)

type SavePoint struct {
	storage *storage.Storage
}

func NewSavePoint(storage *storage.Storage) *SavePoint {
	return &SavePoint{storage}
}

func (s *SavePoint) Load() (lastBlock uint64, lastTxIndex, lastLogIndex uint, err error) {
	globals, err := s.storage.Globals()
	if err != nil {
		return 0, 0, 0, err
	}
	return globals.LastBlock, globals.LastTxIndex, globals.LastLogIndex, nil
}

func (s *SavePoint) Save(logevent *types.Log) error {
	globals, err := s.storage.Globals()
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

	return s.storage.SetGlobals(*globals)
}

func (s *SavePoint) SkipTx(txid common.Hash) (bool, error) {
	return s.storage.SkipTx(txid)
}
