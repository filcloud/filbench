package cmd

import (
	"fmt"
	"github.com/mitchellh/go-homedir"
	"os"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var repoDir string
var filbenchDir string

const filbenchDirEnvVar = "FILBENCH_DIR"
const defaultFilbenchDir = "~/.filbench"

func getFilbenchDir() string {
	var dir string
	if filbenchDir != "" {
		dir = filbenchDir // command line flag
	} else {
		dir = os.Getenv(filbenchDirEnvVar) // environment variable
		if dir == "" {
			dir = defaultFilbenchDir // default
		}
	}
	dir, err := homedir.Expand(dir)
	if err != nil {
		panic(err)
	}
	return dir
}

func init() {
	rootCmd.PersistentFlags().StringVar(&repoDir, "repodir", "", "The directory of the filecoin repo")
	rootCmd.PersistentFlags().StringVar(&filbenchDir, "filbenchdir", "", "The directory of the filbench metadata")
}

var rootCmd = &cobra.Command{
	Use:   "filbench",
	Short: "filbench - Filecoin benchmarking tool",
	Long:  ``,
	Version: "0.0.1",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

var black = color.New(color.FgBlack).SprintFunc()
var red = color.New(color.FgRed).SprintFunc()
var green = color.New(color.FgGreen).SprintFunc()
var yellow = color.New(color.FgYellow).SprintFunc()
var blue = color.New(color.FgBlue).SprintFunc()
var magenta = color.New(color.FgMagenta).SprintFunc()
var cyan = color.New(color.FgCyan).SprintFunc()
var white = color.New(color.FgWhite).SprintFunc()
