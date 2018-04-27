package cmd

import (
	"fmt"
	"os"

	cfg "github.com/adriamb/goic/config"

	log "github.com/sirupsen/logrus"
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
	Run:   cmdServe,
}

var deployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy the smartcontracts",
	Long:  "Deploy the smartcontracts in two chains",
	Run:   cmdDeploy,
}

var addHashCmd = &cobra.Command{
	Use:   "addhash",
	Short: "Add hash",
	Long:  "addhash <ipfshash> <ttl>",
	Run:   cmdAddHash,
}

var rmHashCmd = &cobra.Command{
	Use:   "rmhash",
	Short: "Remove a hash",
	Long:  "rmhash <ipfshash>",
	Run:   cmdRemoveHash,
}

var dumpDbCmd = &cobra.Command{
	Use:   "dumpdb",
	Short: "Dumps the database",
	Long:  "Dumps the database",
	Run:   cmdDumpDb,
}
var initDbCmd = &cobra.Command{
	Use:   "initdb",
	Short: "Dumps the database",
	Long:  "Dumps the database",
	Run:   cmdInitDb,
}

var skipTxCmd = &cobra.Command{
	Use:   "skiptx",
	Short: "Skip a transaction",
	Long:  "Do not process the selected transaction",
	Run:   cmdAddSkipTx,
}

// ExecuteCmd adds all child commands to the root command sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func ExecuteCmd() {

	log.SetLevel(log.DebugLevel)

	if err := RootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(-1)

	}
}

func init() {

	//jww.SetLogThreshold(jww.LevelTrace)
	//jww.SetStdoutThreshold(jww.LevelTrace)

	cobra.OnInitialize(initConfig)
	RootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/goic.yaml)")
	RootCmd.PersistentFlags().IntVar(&cfg.Verbose, "verbose", 0, "verboose level")
	RootCmd.AddCommand(serveCmd)
	RootCmd.AddCommand(deployCmd)
	RootCmd.AddCommand(addHashCmd)
	RootCmd.AddCommand(rmHashCmd)
	RootCmd.AddCommand(dumpDbCmd)
	RootCmd.AddCommand(initDbCmd)
	RootCmd.AddCommand(skipTxCmd)
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {

	viper.SetConfigType("yaml")
	viper.SetConfigName("goic")  // name ofconfig file (without extension)
	viper.AddConfigPath(".")     // adding current directory as first search path
	viper.AddConfigPath("$HOME") // adding home directory as first search path
	viper.SetEnvPrefix("GOIC")   // so viper.AutomaticEnv will get matching envvars starting with O2M_
	viper.AutomaticEnv()         // read in environment variables that match

	if cfgFile != "" { // enable ability to specify config file via flag
		viper.SetConfigFile(cfgFile)
	}

	// If a config file is found, read it in.

	if err := viper.ReadInConfig(); err == nil {
		log.Println("Using config file:", viper.ConfigFileUsed())
		if err := viper.Unmarshal(&cfg.C); err != nil {
			panic(err)
		}
	} else {
		log.Fatalln(err)
	}

}
