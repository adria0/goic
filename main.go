package main

import (
	banner "github.com/CrowdSurge/banner"
	cmd "github.com/adriamb/goic/cmd"
)

func main() {
	banner.Print("goic")
	cmd.ExecuteCmd()
}
