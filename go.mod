module github.com/filcloud/filbench

go 1.12

require (
	github.com/fatih/color v1.7.0
	github.com/filecoin-project/go-filecoin v0.0.1
	github.com/filecoin-project/go-leb128 v0.0.0-20190212224330-8d79a5489543
	github.com/filecoin-project/go-sectorbuilder v0.0.0-20190801004428-e75bc9b0aaea
	github.com/ipfs/go-blockservice v0.0.2
	github.com/ipfs/go-cid v0.0.3
	github.com/ipfs/go-datastore v0.0.5
	github.com/ipfs/go-ds-badger v0.0.5
	github.com/ipfs/go-ipfs-blockstore v0.0.1
	github.com/ipfs/go-ipfs-exchange-offline v0.0.1
	github.com/ipfs/go-ipfs-keystore v0.0.1
	github.com/ipfs/go-ipld-cbor v0.0.3
	github.com/ipfs/go-ipld-format v0.0.1
	github.com/ipfs/go-merkledag v0.0.2
	github.com/libp2p/go-libp2p-crypto v0.1.0
	github.com/mitchellh/go-homedir v1.1.0
	github.com/multiformats/go-multibase v0.0.1
	github.com/multiformats/go-multihash v0.0.6
	github.com/pkg/errors v0.8.1
	github.com/spf13/cobra v0.0.5
)

replace github.com/filecoin-project/go-filecoin => ../../filecoin-project/go-filecoin

replace github.com/filecoin-project/go-bls-sigs => ../../filecoin-project/go-filecoin/go-bls-sigs

replace github.com/filecoin-project/go-sectorbuilder => ../../filecoin-project/go-filecoin/go-sectorbuilder
