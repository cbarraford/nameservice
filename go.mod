module github.com/cbarraford/nameservice

go 1.12

require (
	github.com/cosmos/cosmos-sdk v0.35.0
	github.com/cosmos/sdk-application-tutorial v0.0.0-20190613140616-dae561a27282
	github.com/gogo/protobuf v1.1.1
	github.com/gorilla/mux v1.7.0
	github.com/spf13/cobra v0.0.3
	github.com/spf13/viper v1.0.3
	github.com/tendermint/go-amino v0.15.0
	github.com/tendermint/tendermint v0.31.5
)

replace golang.org/x/crypto => github.com/tendermint/crypto v0.0.0-20180820045704-3764759f34a5

replace github.com/cosmos/cosmos-sdk => github.com/cosmos/cosmos-sdk v0.28.2-0.20190605232616-5f9c3fdf8895
