package main

import (
	"fmt"

	banner "github.com/CrowdSurge/banner"
	cmd "github.com/adriamb/goic/cmd"
)

func main() {
	banner.Print("goic")
	fmt.Println("IPFS Consortium go implementation.")
	cmd.ExecuteCmd()
}
