package cmd

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	"github.com/ipfs/go-datastore/query"
	badgerds "github.com/ipfs/go-ds-badger"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	cbor "github.com/ipfs/go-ipld-cbor"
	format "github.com/ipfs/go-ipld-format"
	"github.com/ipfs/go-merkledag"
	"github.com/spf13/cobra"

	"github.com/filecoin-project/go-filecoin/address"
	"github.com/filecoin-project/go-filecoin/plumbing/dag"
	"github.com/filecoin-project/go-filecoin/proofs/sectorbuilder"
	"github.com/filecoin-project/go-filecoin/proofs/verification"
	"github.com/filecoin-project/go-filecoin/repo"
	"github.com/filecoin-project/go-filecoin/types"
	go_sectorbuilder "github.com/filecoin-project/go-sectorbuilder"
)

var pieceNum int

var minerAddr address.Address

func init() {
	rootCmd.AddCommand(SectorBuilderCmd)

	SectorBuilderCmd.AddCommand(SectorBuilderAddPieceCmd)
	SectorBuilderCmd.AddCommand(SectorBuilderGenPieceCmd)
	SectorBuilderCmd.AddCommand(SectorBuilderLsPiecesCmd)
	SectorBuilderCmd.AddCommand(SectorBuilderGetPiecesCmd)
	SectorBuilderCmd.AddCommand(SectorBuilderSealSectorsCmd)
	SectorBuilderCmd.AddCommand(SectorBuilderLsSectorsCmd)
	SectorBuilderCmd.AddCommand(SectorBuilderVerifySectorsPorepCmd)
	SectorBuilderCmd.AddCommand(SectorBuilderVerifySectorsPostCmd)

	SectorBuilderGenPieceCmd.Flags().IntVarP(&pieceNum, "piece-num", "n", 1, "The number of pieces to generate")

	var err error
	minerAddr, err = address.NewActorAddress([]byte("filbenchminer"))
	if err != nil {
		panic(err)
	}
}

var SectorBuilderCmd = &cobra.Command{
	Use:   "sector-builder",
	Short: "Commands for filecoin sector builder",
}

type DAG struct {
	dag          *dag.DAG
	dagService   format.DAGService
	blockService blockservice.BlockService
	datastore    repo.Datastore
}

func (d *DAG) Close() {
	err := d.blockService.Close()
	if err != nil {
		panic(err)
	}
	err = d.datastore.Close()
	if err != nil {
		panic(err)
	}
}

func openSectorBuilderPiecesDAG() *DAG {
	options := &badgerds.DefaultOptions
	piecesStore, err := badgerds.NewDatastore(filepath.Join(getFilbenchDir(), "pieces"), options)
	if err != nil {
		panic(err)
	}
	blockStore := blockstore.NewBlockstore(piecesStore)
	blockService := blockservice.New(blockStore, offline.Exchange(blockStore))
	dagService := merkledag.NewDAGService(blockService)
	d := dag.NewDAG(dagService)
	return &DAG{
		dag:          d,
		dagService:   dagService,
		blockService: blockService,
		datastore:    piecesStore,
	}
}

var SectorBuilderLsPiecesCmd = &cobra.Command{
	Use:   "ls-pieces",
	Short: "List all pieces",
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		defer func() {
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}()

		ds := openMetaDatastore()
		defer ds.Close()
		dag := openSectorBuilderPiecesDAG()
		defer dag.Close()

		result, err := ds.Query(query.Query{
			Prefix:   metaSectorBuilderPiecePrefix,
			KeysOnly: true,
		})
		if err != nil {
			return
		}

		for entry := range result.Next() {
			err = entry.Error
			if err != nil {
				return
			}
			var c cid.Cid
			c, err = cid.Parse(strings.TrimLeft(entry.Key, metaSectorBuilderPiecePrefix+"/"))

			r, err := dag.dag.Cat(context.Background(), c)
			if err != nil {
				return
			}

			var node format.Node
			node, err = dag.dagService.Get(context.Background(), c)
			if err != nil {
				return
			}
			err = traverseNode(dag.dagService, node, 0, func(node format.Node, depth int) error {
				stat, err := node.Stat()
				if err != nil {
					return err
				}
				s := fmt.Sprintf("data size: %d, links: %d, cumulative size: %d", stat.DataSize, stat.NumLinks, stat.CumulativeSize)
				if depth == 0 {
					fmt.Printf("Piece: %s, %s, original data size: %d\n", blue(node.Cid().String()), s, r.Size())
				} else {
					fmt.Printf("%s%s, %s\n", strings.Repeat("  ", depth), red(node.Cid().String()), s)
				}
				return nil
			})
			if err != nil {
				return
			}
		}

		fmt.Println("Note: data size is node contained bytes (greater than the original data size), cumulative size is node size plus its all children size.")
	},
}

func traverseNode(dagService format.DAGService, node format.Node, depth int, visit func(format.Node, int) error) error {
	err := visit(node, depth)
	if err != nil {
		return err
	}
	for _, link := range node.Links() {
		child, err := dagService.Get(context.Background(), link.Cid)
		if err != nil {
			return err
		}
		err = traverseNode(dagService, child, depth+1, visit)
		if err != nil {
			return err
		}
	}
	return nil
}

var SectorBuilderGetPiecesCmd = &cobra.Command{
	Use:   "get-piece <cid> <file>",
	Short: "Get piece and save into file",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		var err error
		defer func() {
			if err != nil {
				fmt.Println(err)
				os.Exit(1)
			}
		}()

		dag := openSectorBuilderPiecesDAG()
		defer dag.Close()

		c, err := cid.Parse(args[0])
		if err != nil {
			return
		}

		r, err := dag.dag.Cat(context.Background(), c)
		if err != nil {
			return
		}
		filename := args[0]
		if len(args) == 2 {
			filename = args[1]
		}
		f, err := os.OpenFile(filename, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
		if err != nil {
			return
		}
		n, err := io.Copy(f, r)
		if err == nil && uint64(n) < r.Size() {
			err = io.ErrShortWrite
		}
		if err1 := f.Close(); err == nil {
			err = err1
		}
	},
}

var SectorBuilderAddPieceCmd = &cobra.Command{
	Use:   "add-piece <file>",
	Short: "Add piece",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		filename := args[0]
		file, err := os.Open(filename)
		if err != nil {
			panic(err)
		}
		defer file.Close()

		dag := openSectorBuilderPiecesDAG()
		defer dag.Close()
		ds := openMetaDatastore()

		nd, err := dag.dag.ImportData(context.Background(), file)
		if err != nil {
			panic(err)
		}
		err = ds.Put(makeKey(metaSectorBuilderPiecePrefix, nd.Cid().String()), nil)
		if err != nil {
			return
		}

		ds.Close()

		sb := openSectorBuilder()
		defer sb.Close()

		r, err := dag.dag.Cat(context.Background(), nd.Cid())
		if err != nil {
			return
		}
		t := time.Now()
		sectorID, err := sb.AddPiece(context.Background(), nd.Cid(), r.Size(), r)
		if err != nil {
			panic(err)
		}
		fmt.Printf("Added piece %s into staging sector %d, took %v\n", nd.Cid(), sectorID, time.Since(t))

		sb.SealAllStagedUnsealedSectors()
	},
}

var SectorBuilderGenPieceCmd = &cobra.Command{
	Use:   "generate-piece <file>",
	Short: "Generate piece",
	Run: func(cmd *cobra.Command, args []string) {
		sb := openSectorBuilder()
		defer sb.Close()

		for i := 0; i < pieceNum; i++ {
			pieceData := make([]byte, sb.MaxBytesPerSector.Uint64())
			_, err := io.ReadFull(rand.Reader, pieceData)
			if err != nil {
				panic(err)
			}

			data := merkledag.NewRawNode(pieceData)

			t := time.Now()
			sectorID, err := sb.AddPiece(context.Background(), data.Cid(), uint64(len(pieceData)), bytes.NewReader(pieceData))
			if err != nil {
				panic(err)
			}
			fmt.Printf("Generate and add piece %s with size %d into staging sector %d, took %v\n", data.Cid(), len(pieceData), sectorID, time.Since(t))
		}

		sb.SealAllStagedUnsealedSectors()
	},
}

type SectorBuilder struct {
	sectorbuilder.SectorBuilder
	MetaStore         *Datastore
	MaxBytesPerSector *types.BytesAmount
}

func (sb *SectorBuilder) Close() {
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for range sb.SectorSealResults() {
		}
		wg.Done()
	}()
	err := sb.SectorBuilder.Close()
	if err != nil {
		panic(err)
	}
	wg.Wait()
	sb.MetaStore.Close()
}

func (sb *SectorBuilder) SealAllStagedUnsealedSectors() {
	allStaged, err := sb.GetAllStagedSectors()
	if err != nil {
		panic(err)
	}
	var sectorIDs []string
	sectorIDSet := map[uint64]struct{}{}
	for _, s := range allStaged {
		sectorIDs = append(sectorIDs, fmt.Sprint(s.SectorID))
		sectorIDSet[s.SectorID] = struct{}{}
	}

	allSealed := getSealedSectorMetadataList(sb.MetaStore)
	var sealedSectorIDs []string
	for _, s := range allSealed {
		sealedSectorIDs = append(sealedSectorIDs, fmt.Sprint(s.SectorID))
		delete(sectorIDSet, s.SectorID)
	}

	fmt.Println("Seal all staged sectors ...")
	fmt.Printf("  staged sectors: [%s]\n", blue(strings.Join(sectorIDs, ", ")))
	fmt.Printf("  but sealed sectors: [%s]\n", blue(strings.Join(sealedSectorIDs, ", ")))

	if len(sectorIDSet) == 0 {
		fmt.Println("No staged sector needs to seal")
		return
	}

	err = sb.SealAllStagedSectors(context.Background())
	if err != nil {
		panic(err)
	}

	t := time.Now()
	for val := range sb.SectorSealResults() {
		if _, ok := sectorIDSet[val.SectorID]; !ok {
			t = time.Now()
			continue
		}

		sb.HandleSectorSealResult(&val, t)

		delete(sectorIDSet, val.SectorID)
		if len(sectorIDSet) == 0 {
			break
		}

		t = time.Now()
	}
}

func (sb *SectorBuilder) HandleSectorSealResult(r *sectorbuilder.SectorSealResult, startAt time.Time) {
	if r.SealingErr != nil {
		fmt.Printf("Sealing %s: sector %d, took %v, error %s\n",
			red("failed"), r.SectorID, time.Since(startAt), red(r.SealingErr))
	} else if r.SealingResult != nil {
		fmt.Printf("Sealing %s: sector %d, took %v\n", blue("succeeded"), r.SectorID, time.Since(startAt))
		for _, pieceInfo := range r.SealingResult.Pieces {
			fmt.Printf("  Piece %s, size %d\n", cyan(pieceInfo.Ref), pieceInfo.Size)
		}

		sectorIDStr := fmt.Sprint(r.SectorID)
		err := sb.MetaStore.Put(datastore.NewKey(metaSectorBuilderLastUsedSectorIDPrefix), []byte(sectorIDStr))
		if err != nil {
			fmt.Printf("  Saving LastUsedSectorID %d failed\n", r.SectorID)
		}
		bytes, err := cbor.DumpObject(r.SealingResult)
		if err != nil {
			panic(err)
		}
		err = sb.MetaStore.Put(makeKey(metaSectorBuilderSealedSectorMetadataPrefix, sectorIDStr), bytes)
		if err != nil {
			fmt.Printf("  Saving SealedSectorMetadata %d failed\n", r.SectorID)
		}
	}
}

func openSectorBuilder() *SectorBuilder {
	ds := openMetaDatastore()

	var lastUsedSectorID uint64
	v, err := ds.Get(datastore.NewKey(metaSectorBuilderLastUsedSectorIDPrefix))
	if err == nil {
		lastUsedSectorID, err = strconv.ParseUint(string(v), 0, 64)
		if err != nil {
			panic(err)
		}
	} else if err != datastore.ErrNotFound {
		panic(err)
	}

	stagingDir := filepath.Join(getFilbenchDir(), "staging")
	sealedDir := filepath.Join(getFilbenchDir(), "sealed")

	sectorClass := types.NewSectorClass(types.TwoHundredFiftySixMiBSectorSize)

	memRepo := repo.NewInMemoryRepo()
	blockStore := blockstore.NewBlockstore(memRepo.Datastore())
	blockService := blockservice.New(blockStore, offline.Exchange(blockStore))

	sb, err := sectorbuilder.NewRustSectorBuilder(sectorbuilder.RustSectorBuilderConfig{
		BlockService:     blockService, // not used, so just memory repo
		LastUsedSectorID: lastUsedSectorID,
		MetadataDir:      filepath.Join(getFilbenchDir(), "metadata"),
		MinerAddr:        minerAddr,
		SealedSectorDir:  sealedDir,
		SectorClass:      sectorClass,
		StagedSectorDir:  stagingDir,
	})
	if err != nil {
		panic(err)
	}

	max := types.NewBytesAmount(go_sectorbuilder.GetMaxUserBytesPerStagedSector(sectorClass.SectorSize().Uint64()))

	return &SectorBuilder{
		SectorBuilder:     sb,
		MetaStore:         ds,
		MaxBytesPerSector: max,
	}
}

var SectorBuilderSealSectorsCmd = &cobra.Command{
	Use:   "seal-sectors",
	Short: "Seal all staged sectors",
	Run: func(cmd *cobra.Command, args []string) {
		sb := openSectorBuilder()
		defer sb.Close()

		sb.SealAllStagedUnsealedSectors()
	},
}

type SealedSectorMetadataOrder struct {
}

func (o SealedSectorMetadataOrder) Compare(a, b query.Entry) int {
	aID, err := strconv.ParseUint(strings.TrimLeft(a.Key, metaSectorBuilderSealedSectorMetadataPrefix+"/"), 0, 64)
	if err != nil {
		panic(err)
	}
	bID, err := strconv.ParseUint(strings.TrimLeft(a.Key, metaSectorBuilderSealedSectorMetadataPrefix+"/"), 0, 64)
	if err != nil {
		panic(err)
	}
	if aID < bID {
		return -1
	} else if aID > bID {
		return 1
	} else {
		return 0
	}
}

func getSealedSectorMetadataList(metaStore *Datastore) []*sectorbuilder.SealedSectorMetadata {
	queryResult, err := metaStore.Query(query.Query{
		Prefix: metaSectorBuilderSealedSectorMetadataPrefix,
		Orders: []query.Order{SealedSectorMetadataOrder{}},
	})
	if err != nil {
		panic(err)
	}
	var result []*sectorbuilder.SealedSectorMetadata
	for entry := range queryResult.Next() {
		err = entry.Error
		if err != nil {
			panic(err)
		}

		var m sectorbuilder.SealedSectorMetadata
		err = cbor.DecodeInto(entry.Value, &m)
		if err != nil {
			panic(err)
		}
		result = append(result, &m)
	}
	return result
}

var SectorBuilderVerifySectorsPorepCmd = &cobra.Command{
	Use:   "verify-sectors-porep",
	Short: "Verify PoRep (Proof-of-Replication) of all sealed sectors",
	Run: func(cmd *cobra.Command, args []string) {
		sb := openSectorBuilder()
		defer sb.Close()

		allSealed := getSealedSectorMetadataList(sb.MetaStore)
		var sectorIDs []string
		for _, s := range allSealed {
			sectorIDs = append(sectorIDs, fmt.Sprint(s.SectorID))
		}
		fmt.Printf("All sealed sectors: [%s]\n", blue(strings.Join(sectorIDs, ", ")))
		for _, s := range allSealed {
			fmt.Printf("Verify sector %d: ", s.SectorID)
			t := time.Now()
			res, err := (&verification.RustVerifier{}).VerifySeal(verification.VerifySealRequest{
				CommD:      s.CommD,
				CommR:      s.CommR,
				CommRStar:  s.CommRStar,
				Proof:      s.Proof,
				ProverID:   sectorbuilder.AddressToProverID(minerAddr),
				SectorID:   s.SectorID,
				SectorSize: types.TwoHundredFiftySixMiBSectorSize,
			})
			if err != nil {
				fmt.Printf("error %s", red(err))
			} else if !res.IsValid {
				fmt.Print(red("invalid"))
			} else {
				fmt.Print("valid")
			}
			fmt.Printf(", took %v\n", time.Since(t))
		}
	},
}

var SectorBuilderLsSectorsCmd = &cobra.Command{
	Use:   "ls-sectors",
	Short: "List all sectors",
	Run: func(cmd *cobra.Command, args []string) {
		sb := openSectorBuilder()
		defer sb.Close()

		fmt.Println(green("Staged sectors:"))
		allStaged, err := sb.GetAllStagedSectors()
		if err != nil {
			panic(err)
		}
		for _, s := range allStaged {
			fmt.Printf("  Sector %d\n", s.SectorID)
		}

		fmt.Println(green("Sealed sectors:"))
		allSealed := getSealedSectorMetadataList(sb.MetaStore)
		for _, s := range allSealed {
			fmt.Printf("  Sector %d\n", s.SectorID)
			for _, p := range s.Pieces {
				fmt.Printf("    Piece %s, size %d\n", cyan(p.Ref), p.Size)
			}
		}
	},
}

var SectorBuilderVerifySectorsPostCmd = &cobra.Command{
	Use:   "verify-sectors-post",
	Short: "Challenge and verify PoSt (Proof-of-Spacetime) of all sealed sectors",
	Run: func(cmd *cobra.Command, args []string) {
		var challengeSeed types.PoStChallengeSeed
		_, err := io.ReadFull(rand.Reader, challengeSeed[:])
		if err != nil {
			panic(err)
		}
		fmt.Printf("Use challenge seed: %s\n", hex.EncodeToString(challengeSeed[:]))

		sb := openSectorBuilder()
		defer sb.Close()

		allSealed := getSealedSectorMetadataList(sb.MetaStore)
		var sectorInfos []go_sectorbuilder.SectorInfo
		var sectorIDs []string
		for _, s := range allSealed {
			sectorInfos = append(sectorInfos, go_sectorbuilder.SectorInfo{
				SectorID: s.SectorID,
				CommR:    s.CommR,
			})
			sectorIDs = append(sectorIDs, fmt.Sprint(s.SectorID))
		}
		sortedSectorInfo := go_sectorbuilder.NewSortedSectorInfo(sectorInfos...)

		fmt.Println("Generate PoSt ...")
		t := time.Now()
		gres, err := sb.GeneratePoSt(sectorbuilder.GeneratePoStRequest{
			SortedSectorInfo: sortedSectorInfo,
			ChallengeSeed:    challengeSeed,
		})
		if err != nil {
			panic(err)
		}
		fmt.Printf("  sectors: [%s]\n", blue(strings.Join(sectorIDs, ", ")))
		fmt.Printf("  proof %s, took %v\n", hex.EncodeToString(gres.Proof), time.Since(t))

		fmt.Println("Verify PoSt ...")
		t = time.Now()
		vres, err := (&verification.RustVerifier{}).VerifyPoSt(verification.VerifyPoStRequest{
			ChallengeSeed:    challengeSeed,
			SortedSectorInfo: sortedSectorInfo,
			Faults:           []uint64{},
			Proof:            gres.Proof,
			SectorSize:       types.TwoHundredFiftySixMiBSectorSize,
		})
		if err != nil {
			panic(err)
		}
		if !vres.IsValid {
			fmt.Print(red("  invalid"))
		} else {
			fmt.Print("  valid")
		}
		fmt.Printf(", took %v\n", time.Since(t))
	},
}
