package gometh

import (
	"fmt"

	"github.com/ethereum/go-ethereum/common"
)

var Verbose int

// C is the package config
var C Config

// Config is the server configurtion
type Config struct {
	Keystore struct {
		Account string
		Path    string
		Passwd  string
	}

	Contracts struct {
		IPFSProxy struct {
			JSONURL string
			Address string
			Deploy  struct {
				Members      []string
				Required     uint
				PersistLimit uint
			}
		}
	}
	IPFS struct {
		APIURL string
	}

	Web3 struct {
		RPCURL string
	}
}

func (c *Config) Verify() error {

	if !common.IsHexAddress(c.Contracts.IPFSProxy.Address) {
		return fmt.Errorf("Bad Address %v", c.Contracts.IPFSProxy.Address)
	}
	return nil

}
