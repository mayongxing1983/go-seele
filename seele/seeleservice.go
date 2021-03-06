/**
*  @file
*  @copyright defined in go-seele/LICENSE
 */

package seele

import (
	"context"
	"path/filepath"

	"github.com/seeleteam/go-seele/common"
	"github.com/seeleteam/go-seele/core"
	"github.com/seeleteam/go-seele/core/store"
	"github.com/seeleteam/go-seele/core/types"
	"github.com/seeleteam/go-seele/database"
	"github.com/seeleteam/go-seele/database/leveldb"
	"github.com/seeleteam/go-seele/log"
	"github.com/seeleteam/go-seele/p2p"
	"github.com/seeleteam/go-seele/rpc"
	"github.com/seeleteam/go-seele/seele/download"
)

// SeeleService implements full node service.
type SeeleService struct {
	networkID     uint64
	p2pServer     *p2p.Server
	seeleProtocol *SeeleProtocol
	log           *log.SeeleLog
	Coinbase      common.Address // account address that mining rewards will be send to.

	txPool         *core.TransactionPool
	chain          *core.Blockchain
	chainDB        database.Database // database used to store blocks.
	accountStateDB database.Database // database used to store account state info.
}

// ServiceContext is a collection of service configuration inherited from node
type ServiceContext struct {
	DataDir string
}

func (s *SeeleService) TxPool() *core.TransactionPool { return s.txPool }
func (s *SeeleService) BlockChain() *core.Blockchain  { return s.chain }
func (s *SeeleService) NetVersion() uint64            { return s.networkID }
func (s *SeeleService) Downloader() *downloader.Downloader {
	return s.seeleProtocol.Downloader()
}

// ApplyTransaction applys a transaction
// Check if this transaction is valid in the state db
func (s *SeeleService) ApplyTransaction(coinbase common.Address, tx *types.Transaction) error {
	// TODO
	return nil
}

// NewSeeleService create SeeleService
func NewSeeleService(ctx context.Context, conf *Config, log *log.SeeleLog) (s *SeeleService, err error) {
	s = &SeeleService{
		networkID: conf.NetworkID,
		log:       log,
	}
	s.Coinbase = conf.Coinbase
	serviceContext := ctx.Value("ServiceContext").(ServiceContext)

	// Initialize blockchain DB.
	chainDBPath := filepath.Join(serviceContext.DataDir, BlockChainDir)
	log.Info("NewSeeleService BlockChain datadir is %s", chainDBPath)
	s.chainDB, err = leveldb.NewLevelDB(chainDBPath)
	if err != nil {
		log.Error("NewSeeleService Create BlockChain err. %s", err)
		return nil, err
	}

	// Initialize account state info DB.
	accountStateDBPath := filepath.Join(serviceContext.DataDir, AccountStateDir)
	log.Info("NewSeeleService account state datadir is %s", accountStateDBPath)
	s.accountStateDB, err = leveldb.NewLevelDB(accountStateDBPath)
	if err != nil {
		s.chainDB.Close()
		log.Error("NewSeeleService Create BlockChain err: failed to create account state DB, %s", err)
		return nil, err
	}

	bcStore := store.NewBlockchainDatabase(s.chainDB)
	genesis := core.DefaultGenesis(bcStore)
	err = genesis.Initialize(s.accountStateDB)
	if err != nil {
		s.chainDB.Close()
		s.accountStateDB.Close()
		log.Error("NewSeeleService genesis.Initialize err. %s", err)
		return nil, err
	}

	s.chain, err = core.NewBlockchain(bcStore, s.accountStateDB)
	if err != nil {
		s.chainDB.Close()
		s.accountStateDB.Close()
		log.Error("NewSeeleService init chain failed. %s", err)
		return nil, err
	}

	s.txPool = core.NewTransactionPool(conf.TxConf, s.chain)
	s.seeleProtocol, err = NewSeeleProtocol(s, log)
	if err != nil {
		s.chainDB.Close()
		s.accountStateDB.Close()
		log.Error("NewSeeleService create seeleProtocol err. %s", err)
		return nil, err
	}

	return s, nil
}

// Protocols implements node.Service, returning all the currently configured
// network protocols to start.
func (s *SeeleService) Protocols() (protos []p2p.Protocol) {
	protos = append(protos, s.seeleProtocol.Protocol)
	return
}

// Start implements node.Service, starting goroutines needed by SeeleService.
func (s *SeeleService) Start(srvr *p2p.Server) error {
	s.p2pServer = srvr

	s.seeleProtocol.Start()
	return nil
}

// Stop implements node.Service, terminating all internal goroutines.
func (s *SeeleService) Stop() error {
	s.seeleProtocol.Stop()

	//TODO
	// s.txPool.Stop() s.chain.Stop()
	// retries? leave it to future
	s.chainDB.Close()
	s.accountStateDB.Close()
	return nil
}

// APIs implements node.Service, returning the collection of RPC services the seele package offers.
func (s *SeeleService) APIs() (apis []rpc.API) {
	return append(apis, []rpc.API{
		{
			Namespace: "seele",
			Version:   "1.0",
			Service:   NewPublicSeeleAPI(s),
			Public:    true,
		},
		{
			Namespace: "download",
			Version:   "1.0",
			Service:   downloader.NewPublicdownloaderAPI(s.seeleProtocol.downloader),
			Public:    true,
		},
		{
			Namespace: "network",
			Version:   "1.0",
			Service:   NewPublicNetworkAPI(s.p2pServer, s.NetVersion()),
			Public:    true,
		},
	}...)
}
