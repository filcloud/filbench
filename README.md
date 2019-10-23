# filbench

Filecoin benchmarking tool

## Background

Filecoin is currently under active development by the Protocol Labs, and developers or miners can use [go-filecoin](https://github.com/filecoin-project/go-filecoin) or [lotus](https://github.com/filecoin-project/lotus) to participate in the devnet. However, there is a lack of a utility that can quickly test some of Filecoin's key algorithms (such as PoRep and PoSt), which is not conducive to the miners evaluating the performance of commercial mining hardware as good as the manufacturer claims. Therefore, a simple and practical benchmarking tool **filbench** has been developed by us, hoping to help developers and miners to a certain extent.

Filecoin 目前正由协议实验室紧张开发中，开发者或矿工可以使用 `go-filecoin` 或 `lotus` 来参与开发者测试网。然而缺少一款可以迅速测试 Filecoin 的某些关键算法（比如复制证明和时空证明）的实用工具，这也不利于矿工评测商业矿机的性能是否如厂家宣称的那样棒。因此，filbench 这样一个简洁实用的基准测试工具就被我们开发了，希望在一定程度上能够帮助开发者和矿工。

## Install

This project depends on [go-filecoin](https://github.com/filecoin-project/go-filecoin) now, so you should ensure that it has been in your `GOPATH` directory.

Then, install filbench:

```sh
$ mkdir -p ${GOPATH}/src/github.com/filcloud
$ cd ${GOPATH}/src/github.com/filcloud
$ git clone https://github.com/filcloud/filbench.git
$ cd filbench
$ go install
```

## Usage

Initialize filbench repo directory (default to ~/.filbench):

```sh
$ filbench init
```

Generate several pieces of 254MB (here is 2), and seal them into sectors of 256MB (i.e. PoRep):

```sh
$ filbench sector-builder generate-piece --piece-num 2
```

Verify the results of PoRep above:

```sh
$ filbench sector-builder verify-sectors-porep
```

Generate PoSt of those sealed sectors, and verify them:

```sh
$ filbench sector-builder verify-sectors-post
```

## Maintainers

[@RideWindX](https://github.com/ridewindx).

## Contributing

Feel free to dive in! [Open an issue](https://github.com/filcloud/filbench/issues/new) or submit PRs.

## License

[MIT](LICENSE) © FilCloud
