/**
*  @file
*  @copyright defined in go-seele/LICENSE
 */

package seele

import (
	"math/big"

	"github.com/seeleteam/go-seele/common"
	"github.com/seeleteam/go-seele/common/hexutil"
	"github.com/seeleteam/go-seele/core/types"
	"github.com/seeleteam/go-seele/p2p"
)

// PublicSeeleAPI provides an API to access full node-related information.
type PublicSeeleAPI struct {
	s *SeeleService
}

// NewPublicSeeleAPI creates a new PublicSeeleAPI object for rpc service.
func NewPublicSeeleAPI(s *SeeleService) *PublicSeeleAPI {
	return &PublicSeeleAPI{s}
}

// MinerInfo miner simple info
type MinerInfo struct {
	Coinbase           common.Address
	CurrentBlockHeight uint64
	HeaderHash         common.Hash
}

// GetInfo gets the account address that mining rewards will be send to.
func (api *PublicSeeleAPI) GetInfo(input interface{}, info *MinerInfo) error {
	block, _ := api.s.chain.CurrentBlock()

	*info = MinerInfo{
		Coinbase:           api.s.Coinbase,
		CurrentBlockHeight: block.Header.Height,
		HeaderHash:         block.HeaderHash,
	}

	return nil
}

// GetBalance get balance of the account. if the account's address is empty, will get the coinbase balance
func (api *PublicSeeleAPI) GetBalance(account *common.Address, result *big.Int) error {
	if account == nil || account.Equal(common.Address{}) {
		*account = api.s.Coinbase
	}

	state := api.s.chain.CurrentState()
	balance := state.GetBalance(*account)
	result.Set(balance)
	return nil
}

// AddTx add a tx to miner
func (api *PublicSeeleAPI) AddTx(tx *types.Transaction, result *bool) error {
	err := api.s.txPool.AddTransaction(tx)
	if err != nil {
		*result = false
		return err
	}

	*result = true
	return nil
}

// GetAccountNonce get account next used nonce
func (api *PublicSeeleAPI) GetAccountNonce(account *common.Address, nonce *uint64) error {
	state := api.s.chain.CurrentState()
	*nonce = state.GetNonce(*account)

	return nil
}

// GetBlockHeight get the block height of the chain head
func (api *PublicSeeleAPI) GetBlockHeight(input interface{}, height *uint64) error {
	block, _ := api.s.chain.CurrentBlock()
	*height = block.Header.Height

	return nil
}

// GetBlockByHeightRequest request param for GetBlockByHeight api
type GetBlockByHeightRequest struct {
	Height int64
	FullTx bool
}

// GetBlockByHeight returns the requested block. When blockNr is -1 the chain head is returned. When fullTx is true all
// transactions in the block are returned in full detail, otherwise only the transaction hash is returned
func (api *PublicSeeleAPI) GetBlockByHeight(request *GetBlockByHeightRequest, result *map[string]interface{}) error {
	store := api.s.chain.GetStore()
	var block *types.Block
	if request.Height < 0 {
		block, _ = api.s.chain.CurrentBlock()
	} else {
		hash, err := store.GetBlockHash(uint64(request.Height))
		if err != nil {
			return err
		}
		var er error
		block, er = store.GetBlock(hash)
		if er != nil {
			return er
		}
	}
	response, err := rpcOutputBlock(block, request.FullTx)
	if err != nil {
		return err
	}
	*result = response
	return nil
}

// GetBlockByHashRequest request param for GetBlockByHash api
type GetBlockByHashRequest struct {
	HashHex string
	FullTx  bool
}

// GetBlockByHash returns the requested block. When fullTx is true all transactions in the block are returned in full
// detail, otherwise only the transaction hash is returned
func (api *PublicSeeleAPI) GetBlockByHash(request *GetBlockByHashRequest, result *map[string]interface{}) error {
	store := api.s.chain.GetStore()
	hashByte, err := hexutil.HexToBytes(request.HashHex)
	if err != nil {
		return err
	}
	hash := common.BytesToHash(hashByte)
	block, err := store.GetBlock(hash)
	if err != nil {
		return err
	}
	response, err := rpcOutputBlock(block, request.FullTx)
	if err != nil {
		return err
	}
	*result = response
	return nil
}

// rpcOutputBlock converts the given block to the RPC output which depends on fullTx
func rpcOutputBlock(b *types.Block, fullTx bool) (map[string]interface{}, error) {
	head := b.Header
	fields := map[string]interface{}{
		"height":     head.Height,
		"hash":       b.HeaderHash.ToHex(),
		"parentHash": head.PreviousBlockHash.ToHex(),
		"nonce":      head.Nonce,
		"stateHash":  head.StateHash.ToHex(),
		"txHash":     head.TxHash.ToHex(),
		"creator":    head.Creator.ToHex(),
		"timestamp":  head.CreateTimestamp,
		"difficulty": head.Difficulty,
	}

	formatTx := func(tx *types.Transaction) interface{} {
		return tx.Hash.ToHex()
	}

	if fullTx {
		formatTx = func(tx *types.Transaction) interface{} {
			transaction := map[string]interface{}{
				"hash":         tx.Hash.ToHex(),
				"from":         tx.Data.From.ToHex(),
				"to":           tx.Data.To.ToHex(),
				"amount":       tx.Data.Amount,
				"accountNonce": tx.Data.AccountNonce,
				"payload":      tx.Data.Payload,
				"timestamp":    tx.Data.Timestamp,
			}
			return transaction
		}
	}

	txs := b.Transactions
	transactions := make([]interface{}, len(txs))
	for i, tx := range txs {
		transactions[i] = formatTx(tx)
	}
	fields["transactions"] = transactions

	return fields, nil
}

// PublicNetworkAPI provides an API to access network information.
type PublicNetworkAPI struct {
	p2pServer      *p2p.Server
	networkVersion uint64
}

// NewPublicNetworkAPI creates a new PublicNetworkAPI object for rpc service.
func NewPublicNetworkAPI(p2pServer *p2p.Server, networkVersion uint64) *PublicNetworkAPI {
	return &PublicNetworkAPI{p2pServer, networkVersion}
}

// GetPeerCount returns the count of peers
func (n *PublicNetworkAPI) GetPeerCount(input interface{}, result *int) error {
	*result = n.p2pServer.PeerCount()
	return nil
}

// GetNetworkVersion returns the network version
func (n *PublicNetworkAPI) GetNetworkVersion(input interface{}, result *uint64) error {
	*result = n.networkVersion
	return nil
}
