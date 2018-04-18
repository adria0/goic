package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"

	cfg "github.com/adriamb/goic/config"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string

// RootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "goic",
	Short: "IPFS consortium client",
	Long:  "IPFS consortium client",
	Run: func(cmd *cobra.Command, args []string) {
		_ = cmd.Help()
	},
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the server",
	Long:  "Start the server",
	Run: func(cmd *cobra.Command, args []string) {
		json, _ := json.MarshalIndent(cfg.C, "", "  ")
		log.Println("Efective configuration: " + string(json))
		mustServe()
	},
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the smartcontracts",
	Long:  "Deploy the smartcontracts in two chains",
	Run: func(cmd *cobra.Command, args []string) {
		json, _ := json.MarshalIndent(cfg.C, "", "  ")
		log.Println("Efective configuration: " + string(json))
		mustDeploy()
	},
}

/*
var lockCmd = &cobra.Command{
	Use:   "lock",
	Short: "Lock ethers",
	Long:  "Send ethers to the parentchain->sidechain",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		setContractsAddress()
		assert(callLock(big.NewInt(10)))
	},
}

var burnCmd = &cobra.Command{
	Use:   "unlock",
	Short: "Unlock ethers",
	Long:  "Send ethers to the sidechain->parentchain",
	Run: func(cmd *cobra.Command, args []string) {
		initClient()
		setContractsAddress()
		assert(callBurn(big.NewInt(10)))
	},
}


*/

// ExecuteCmd adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func ExecuteCmd() {
	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)

	}
}

func init() {

	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.goic.yaml)")
	RootCmd.PersistentFlags().IntVar(&cfg.Verbose, "verbose", 0, "verboose level")
	RootCmd.AddCommand(serveCmd)
	RootCmd.AddCommand(deployCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	viper.SetConfigType("yaml")
	viper.SetConfigName("goic")  // name of config file (without extension)
	viper.AddConfigPath("$HOME") // adding home directory as first search path
	viper.SetEnvPrefix("GOIC")   // so viper.AutomaticEnv will get matching envvars starting with O2M_
	viper.AutomaticEnv()         // read in environment variables that match

	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	// If a config file is found, read it in.

	if err := viper.ReadInConfig(); err == nil {
		log.Println("Using config file:", cfgFile)
		if err := viper.Unmarshal(&cfg.C); err != nil {
			panic(err)
		}
	} else {
		log.Fatalln("Configuration file ~/.goic.yaml not found.")
	}

}
