package database

import(
	"fmt"
	"github.com/syndtr/goleveldb/leveldb"
	"github.com/ethereum/go-ethereum/rlp"
	log "github.com/sirupsen/logrus"

)

const (
	prefixHash = "H"
	prefixContract = "H"
)

var (
	errInconsistentSize = fmt.Errorf("Inconsistent db and IPFS datasize")	
)

type Storage struct {
	db *leveldb.DB 
}

type HashContractEntry struct {
	Address string	
	Ttl uint
}

type HashEntry struct {
	Contracts []HashContractEntry
	DataSize uint
}

type ContractEntry struct {
	CurrentQuota uint
	HashCount uint
}

func New() (*Storage, error) {
	db, err := leveldb.OpenFile("./db", nil)
	if err!=nil {
		return nil,err
	}
	return &Storage{db},nil
}

func (db *Storage) AddHash(contract, hash string, ttl,size uint) error {

	key := append([]byte(prefixHash),[]byte(hash)...)
	var entry HashEntry
	
	value,err := db.db.Get(key,nil)
	if err == nil {
		err := rlp.DecodeBytes(value,&entry)
		if err != nil {
			return err
		}
		if size != entry.DataSize {
			return errInconsistentSize
		}
		var contractEntry *HashContractEntry
		for i,v := range entry.Contracts {
			if v.Address == contract {
				contractEntry =  &entry.Contracts[i]
				break
			}
		}
		if contractEntry != nil {
			if contractEntry.Ttl == ttl {
				// exactly the same ttl for hash contract, return
				log.WithField("Hash",hash).Debug("Hash in db already exists with same TTL.")
				return nil
			}
			// update TTL for existing hash in contract
			log.WithField("Hash",hash).Debug("Hash in db already exists with other TTL.")
			contractEntry.Ttl = ttl			
		} else {
			// add a new contract
			log.WithField("Hash",hash).Debug("Hash in db already exists but not in contract.")
			entry.Contracts = append(entry.Contracts, HashContractEntry {
				Address: contract,
				Ttl : ttl,				
			})
		}
	} else {
		// new entry
		log.WithField("Hash",hash).Debug("Hash in db not exists.")
		entry = HashEntry {
			Contracts : []HashContractEntry{
				HashContractEntry {
					Address: contract,
					Ttl : ttl,
				},
			},
			DataSize : size,
		}
	}
	value, err = rlp.EncodeToBytes(entry)
	if err != nil {
		return err
	}

	return db.db.Put(key, value,nil)
}

func (db *Storage) RemoveHash(contract, hash string) error {
	key := append([]byte(prefixHash),[]byte(hash)...)
	var entry HashEntry

	var err error

	value,err := db.db.Get(key,nil)
	if err == nil {
		// does not exist, return
		return nil
	}

	err = rlp.DecodeBytes(value,&entry)
	if err != nil {
		return err
	}
	var contractOffet int = -1
	for i,v := range entry.Contracts {
		if v.Address == contract {
			contractOffet = i
			break
		}
	}
	if contractOffet == -1 {
		// contract is not in this hash, return
		return nil
	}
	if len(entry.Contracts)==1 {
		// the only contract with this hash, delete all entry, return
		return db.db.Delete(key, nil)
	}

	// remove the contract in hash & save
	entry.Contracts[contractOffet] = entry.Contracts[len(entry.Contracts)-1] 
	entry.Contracts = entry.Contracts[:len(entry.Contracts)-1]

	value, err = rlp.EncodeToBytes(entry)
	if err != nil {
		return err
	}

	return db.db.Put(key, value,nil)
}
