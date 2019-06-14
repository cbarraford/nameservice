package app

import (
	"os"

	"github.com/cbarraford/nameservice/x/nameservice"
	"github.com/gogo/protobuf/codec"
	"github.com/tendermint/tendermint/libs/log"

	"github.com/cosmos/cosmos-sdk/x/auth"
	"github.com/cosmos/cosmos-sdk/x/auth/genaccounts"
	"github.com/cosmos/cosmos-sdk/x/bank"
	"github.com/cosmos/cosmos-sdk/x/genutil"
	"github.com/cosmos/cosmos-sdk/x/params"
	"github.com/cosmos/cosmos-sdk/x/slashing"
	"github.com/cosmos/cosmos-sdk/x/staking"

	bam "github.com/cosmos/cosmos-sdk/baseapp"
	sdk "github.com/cosmos/cosmos-sdk/types"
	dbm "github.com/tendermint/tendermint/libs/db"
)

const appName = "nameservice"

var (
	// default home directories for the application CLI
	DefaultCLIHome = os.ExpandEnv("$HOME/.nscli")

	// DefaultNodeHome sets the folder where the applcation data and configuration will be stored
	DefaultNodeHome = os.ExpandEnv("$HOME/.nsd")

	// ModuleBasicManager is in charge of setting up basic module elemnets
	ModuleBasics sdk.ModuleBasicManager
)

// maintains independent module functionality
func init() {
	ModuleBasics = sdk.NewModuleBasicManager(
		genaccounts.AppModuleBasic{},
		genutil.AppModuleBasic{},
		auth.AppModuleBasic{},
		bank.AppModuleBasic{},
		params.AppModuleBasic{},
		nameservice.AppModule{},
		staking.AppModuleBasic{},
		distr.AppModuleBasic{},
		slashing.AppModuleBasic{},
	)
}

type nameServiceApp struct {
	*bam.BaseApp
	cdc *codec.Codec

	// Keys to access the substores
	keyMain          *sdk.KVStoreKey
	keyAccount       *sdk.KVStoreKey
	keyFeeCollection *sdk.KVStoreKey
	keyStaking       *sdk.KVStoreKey
	tkeyStaking      *sdk.TransientStoreKey
	keyDistr         *sdk.KVStoreKey
	tkeyDistr        *sdk.TransientStoreKey
	keyNS            *sdk.KVStoreKey
	keyParams        *sdk.KVStoreKey
	tkeyParams       *sdk.TransientStoreKey
	keySlashing      *sdk.KVStoreKey

	// Keepers
	accountKeeper       auth.AccountKeeper
	bankKeeper          bank.Keeper
	stakingKeeper       staking.Keeper
	slashingKeeper      slashing.Keeper
	distrKeeper         distr.Keeper
	feeCollectionKeeper auth.FeeCollectionKeeper
	paramsKeeper        params.Keeper
	nsKeeper            nameservice.Keeper

	// Module Manager
	mm *sdk.ModuleManager
}

// NewNameServiceApp is a constructor function for nameServiceApp
func NewNameServiceApp(logger log.Logger, db dbm.DB) *nameServiceApp {

	// First define the top level codec that will be shared by the different modules
	cdc := MakeCodec()

	// BaseApp handles interactions with Tendermint through the ABCI protocol
	bApp := bam.NewBaseApp(appName, logger, db, auth.DefaultTxDecoder(cdc))

	// Here you initialize your application with the store keys it requires
	var app = &nameServiceApp{
		BaseApp: bApp,
		cdc:     cdc,

		keyMain:          sdk.NewKVStoreKey(bam.MainStoreKey),
		keyAccount:       sdk.NewKVStoreKey(auth.StoreKey),
		keyStaking:       sdk.NewKVStoreKey(staking.StoreKey),
		tkeyStaking:      sdk.NewTransientStoreKey(staking.TStoreKey),
		keyDistr:         sdk.NewKVStoreKey(distr.StoreKey),
		tkeyDistr:        sdk.NewTransientStoreKey(distr.TStoreKey),
		keyFeeCollection: sdk.NewKVStoreKey(auth.FeeStoreKey),
		keyNS:            sdk.NewKVStoreKey(nameservice.StoreKey),
		keyParams:        sdk.NewKVStoreKey(params.StoreKey),
		tkeyParams:       sdk.NewTransientStoreKey(params.TStoreKey),
	}

	// The ParamsKeeper handles parameter storage for the application
	app.paramsKeeper = params.NewKeeper(app.cdc, app.keyParams, app.tkeyParams, params.DefaultCodespace)
	// Set specific supspaces
	authSubspace := app.paramsKeeper.Subspace(auth.DefaultParamspace)
	bankSupspace := app.paramsKeeper.Subspace(bank.DefaultParamspace)
	stakingSubspace := app.paramsKeeper.Subspace(staking.DefaultParamspace)
	distrSubspace := app.paramsKeeper.Subspace(distr.DefaultParamspace)
	slashingSubspace := app.paramsKeeper.Subspace(slashing.DefaultParamspace)

	// The AccountKeeper handles address -> account lookups
	app.accountKeeper = auth.NewAccountKeeper(
		app.cdc,
		app.keyAccount,
		authSubspace,
		auth.ProtoBaseAccount,
	)

	// The BankKeeper allows you perform sdk.Coins interactions
	app.bankKeeper = bank.NewBaseKeeper(
		app.accountKeeper,
		bankSupspace,
		bank.DefaultCodespace,
	)

	// The FeeCollectionKeeper collects transaction fees and renders them to the fee distribution module
	app.feeCollectionKeeper = auth.NewFeeCollectionKeeper(cdc, app.keyFeeCollection)

	// The staking keeper
	stakingKeeper := staking.NewKeeper(
		app.cdc,
		app.keyStaking,
		app.tkeyStaking,
		app.bankKeeper,
		stakingSubspace,
		staking.DefaultCodespace,
	)

	app.distrKeeper = distr.NewKeeper(
		app.cdc,
		app.keyDistr,
		distrSubspace,
		app.bankKeeper,
		&stakingKeeper,
		app.feeCollectionKeeper,
		distr.DefaultCodespace,
	)

	app.slashingKeeper = slashing.NewKeeper(
		app.cdc,
		app.keySlashing,
		&stakingKeeper,
		slashingSubspace,
		slashing.DefaultCodespace,
	)

	// register the staking hooks
	// NOTE: stakingKeeper above is passed by reference, so that it will contain these hooks
	app.stakingKeeper = *stakingKeeper.SetHooks(
		staking.NewMultiStakingHooks(
			app.distrKeeper.Hooks(),
			app.slashingKeeper.Hooks()),
	)

	// The NameserviceKeeper is the Keeper from the module for this tutorial
	// It handles interactions with the namestore
	app.nsKeeper = nameservice.NewKeeper(
		app.bankKeeper,
		app.keyNS,
		app.cdc,
	)

	app.mm = sdk.NewModuleManager(
		genaccounts.NewAppModule(app.accountKeeper),
		genutil.NewAppModule(app.accountKeeper, app.stakingKeeper, app.BaseApp.DeliverTx),
		auth.NewAppModule(app.accountKeeper, app.feeCollectionKeeper),
		bank.NewAppModule(app.bankKeeper, app.accountKeeper),
		nameservice.NewAppModule(app.nsKeeper, app.bankKeeper),
		distr.NewAppModule(app.distrKeeper),
		slashing.NewAppModule(app.slashingKeeper, app.stakingKeeper),
		staking.NewAppModule(app.stakingKeeper, app.feeCollectionKeeper, app.distrKeeper, app.accountKeeper),
	)

	app.mm.SetOrderBeginBlockers(distr.ModuleName, slashing.ModuleName)
	app.mm.SetOrderEndBlockers(staking.ModuleName)

	// Sets the order of Genesis - Order matters, genutil is to always come last
	app.mm.SetOrderInitGenesis(
		genaccounts.ModuleName,
		distr.ModuleName,
		staking.ModuleName,
		auth.ModuleName,
		bank.ModuleName,
		slashing.ModuleName,
		nameservice.ModuleName,
		genutil.ModuleName,
	)

	// register all module routes and module queriers
	app.mm.RegisterRoutes(app.Router(), app.QueryRouter())

	// The initChainer handles translating the genesis.json file into initial state for the network
	app.SetInitChainer(app.InitChainer)
	app.SetBeginBlocker(app.BeginBlocker)
	app.SetEndBlocker(app.EndBlocker)
	// The AnteHandler handles signature verification and transaction pre-processing
	app.SetAnteHandler(
		auth.NewAnteHandler(
			app.accountKeeper,
			app.feeCollectionKeeper,
			auth.DefaultSigVerificationGasConsumer,
		),
	)

	app.MountStores(
		app.keyMain,
		app.keyAccount,
		app.keyFeeCollection,
		app.keyStaking,
		app.tkeyStaking,
		app.keyDistr,
		app.tkeyDistr,
		app.keySlashing,
		app.keyNS,
		app.keyParams,
		app.tkeyParams,
	)

	err := app.LoadLatestVersion(app.keyMain)
	if err != nil {
		cmn.Exit(err.Error())
	}

	return app
}
