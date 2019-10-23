package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/ipfs/go-datastore"
	badgerds "github.com/ipfs/go-ds-badger"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/filecoin-project/go-filecoin/repo"
)

func init() {
	rootCmd.AddCommand(InitCmd)
}

var InitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a filbench directory",
	Long:  "",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		defer func() {
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}()

		filbenchDir := getFilbenchDir()

		err = repo.EnsureWritableDirectory(filbenchDir)
		if err != nil {
			return
		}
		empty, err := repo.IsEmptyDir(filbenchDir)
		if err != nil {
			err = errors.Wrapf(err, "failed to list filbench directory %s", filbenchDir)
			return
		}
		if !empty {
			err = fmt.Errorf("refusing to initialize filbench in non-empty directory %s", filbenchDir)
			return
		}

		dag := openSectorBuilderPiecesDAG()
		defer dag.Close()
		datastore := openMetaDatastore()
		defer datastore.Close()
	},
}

type Datastore struct {
	repo.Datastore
}

func (d *Datastore) Close() {
	err := d.Datastore.Close()
	if err != nil {
		panic(err)
	}
}

func openMetaDatastore() *Datastore {
	options := &badgerds.DefaultOptions
	d, err := badgerds.NewDatastore(filepath.Join(getFilbenchDir(), "meta"), options)
	if err != nil {
		panic(err)
	}
	return &Datastore{d}
}

const (
	metaSectorBuilderPiecePrefix                = "/piece"
	metaSectorBuilderLastUsedSectorIDPrefix     = "/last-used-sector-id"
	metaSectorBuilderSealedSectorMetadataPrefix = "/sealed-sector-metadata"
)

func makeKey(parts ...string) datastore.Key {
	return datastore.KeyWithNamespaces(parts)
}
