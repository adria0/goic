package cmd

import "log"

func mustDeploy() {

	mustLoad()

	// -- deploy contracts
	_, _, err := contract.DeploySync()
	if err != nil {
		panic(err)
	}

	log.Println("goic contract deployed at ", contract.Address().Hex())

}
