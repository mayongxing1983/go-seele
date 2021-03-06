/**
* @file
* @copyright defined in go-seele/LICENSE
 */

package core

import (
	"bytes"
	"errors"
	"math/big"
	"sync"

	"github.com/seeleteam/go-seele/common"
	"github.com/seeleteam/go-seele/core/state"
	"github.com/seeleteam/go-seele/core/store"
	"github.com/seeleteam/go-seele/core/types"
	"github.com/seeleteam/go-seele/database"
	"github.com/seeleteam/go-seele/miner/pow"
)

var (
	// ErrBlockHashMismatch is returned when the block hash does not match the header hash.
	ErrBlockHashMismatch = errors.New("block header hash mismatch")

	// ErrBlockTxsHashMismatch is returned when the block transactions hash does not match
	// the transaction root hash in the header.
	ErrBlockTxsHashMismatch = errors.New("block transactions root hash mismatch")

	// ErrBlockInvalidParentHash is returned when inserting a new header with invalid parent block hash.
	ErrBlockInvalidParentHash = errors.New("invalid parent block hash")

	// ErrBlockInvalidHeight is returned when inserting a new header with invalid block height.
	ErrBlockInvalidHeight = errors.New("invalid block height")

	// ErrBlockAlreadyExists is returned when inserted block already exists
	ErrBlockAlreadyExists = errors.New("block already exists")

	// ErrBlockStateHashMismatch is returned when the calculated account state hash of block
	// does not match the state root hash in block header.
	ErrBlockStateHashMismatch = errors.New("block state hash mismatch")

	// ErrBlockEmptyTxs is returned when writing a block with empty transactions.
	ErrBlockEmptyTxs = errors.New("empty transactions in block")

	// ErrBlockInvalidToAddress is returned when the to address of miner reward tx is nil.
	ErrBlockInvalidToAddress = errors.New("invalid to address")

	// ErrBlockCoinbaseMismatch is returned when the to address of miner reward tx does not match
	// the creator address in the block header.
	ErrBlockCoinbaseMismatch = errors.New("coinbase mismatch")

	errContractCreationNotSupported = errors.New("smart contract creation not supported yet")
)

type consensusEngine interface {
	// ValidateHeader validates the specified header and return error if validation failed.
	// Generally, need to validate the block nonce.
	ValidateHeader(blockHeader *types.BlockHeader) error

	// ValidateRewardAmount validates the specified amount and returns error if validation failed.
	// The amount of miner reward will change over time.
	ValidateRewardAmount(amount *big.Int) error
}

// Blockchain represents the block chain with a genesis block. The Blockchain manages
// blocks insertion, deletion, reorganizations and persistence with a given database.
// This is a thread safe structure. we must keep all of its parameters are thread safe too.
type Blockchain struct {
	bcStore        store.BlockchainStore
	accountStateDB database.Database
	engine         consensusEngine
	headerChain    *HeaderChain
	genesisBlock   *types.Block
	lock           sync.RWMutex // lock for update blockchain info. for example write block

	blockLeaves *BlockLeaves
}

// NewBlockchain returns an initialized block chain with the given store and account state DB.
func NewBlockchain(bcStore store.BlockchainStore, accountStateDB database.Database) (*Blockchain, error) {
	bc := &Blockchain{
		bcStore:        bcStore,
		accountStateDB: accountStateDB,
		engine:         &pow.Engine{},
	}

	var err error
	bc.headerChain, err = NewHeaderChain(bcStore)
	if err != nil {
		return nil, err
	}

	// Get the genesis block from store
	genesisHash, err := bcStore.GetBlockHash(genesisBlockHeight)
	if err != nil {
		return nil, err
	}

	bc.genesisBlock, err = bcStore.GetBlock(genesisHash)
	if err != nil {
		return nil, err
	}

	// Get the HEAD block from store
	currentHeaderHash, err := bcStore.GetHeadBlockHash()
	if err != nil {
		return nil, err
	}

	currentBlock, err := bcStore.GetBlock(currentHeaderHash)
	if err != nil {
		return nil, err
	}

	td, err := bcStore.GetBlockTotalDifficulty(currentHeaderHash)
	if err != nil {
		return nil, err
	}

	// Get the state DB of the current block
	currentState, err := state.NewStatedb(currentBlock.Header.StateHash, accountStateDB)
	if err != nil {
		return nil, err
	}

	blockIndex := NewBlockIndex(currentState, currentBlock, td)
	bc.blockLeaves = NewBlockLeaves()
	bc.blockLeaves.Add(blockIndex)

	return bc, nil
}

// CurrentBlock returns the HEAD block of the blockchain.
func (bc *Blockchain) CurrentBlock() (*types.Block, *state.Statedb) {
	bc.lock.RLock()
	defer bc.lock.RUnlock()

	index := bc.blockLeaves.GetBestBlockIndex()
	if index == nil {
		return nil, nil
	}

	return index.currentBlock, index.state
}

// CurrentState returns the state DB of the current block.
func (bc *Blockchain) CurrentState() *state.Statedb {
	_, state := bc.CurrentBlock()
	return state
}

// WriteBlock writes the specified block to the blockchain store.
func (bc *Blockchain) WriteBlock(block *types.Block) error {
	// Do not write the block if already exists.
	exist, err := bc.bcStore.HasBlock(block.HeaderHash)
	if err != nil {
		return err
	}

	if exist {
		return ErrBlockAlreadyExists
	}

	bc.lock.Lock()
	defer bc.lock.Unlock()

	var preBlock *types.Block
	if preBlock, err = bc.bcStore.GetBlock(block.Header.PreviousBlockHash); err != nil {
		return ErrBlockInvalidParentHash
	}

	// Ensure the specified block is valid to insert.
	if err = bc.validateBlock(block, preBlock); err != nil {
		return err
	}

	// Process the txs in the block and check the state root hash.
	var blockStatedb *state.Statedb
	if blockStatedb, err = bc.applyTxs(block, preBlock); err != nil {
		return err
	}

	batch := bc.accountStateDB.NewBatch()
	committed := false
	defer func() {
		if !committed {
			batch.Rollback()
		}
	}()

	var stateRootHash common.Hash
	stateRootHash = blockStatedb.Commit(batch)

	if !stateRootHash.Equal(block.Header.StateHash) {
		return ErrBlockStateHashMismatch
	}

	// Update block leaves and write the block into store.
	currentBlock := &types.Block{
		HeaderHash:   block.HeaderHash,
		Header:       block.Header.Clone(),
		Transactions: make([]*types.Transaction, len(block.Transactions)),
	}
	copy(currentBlock.Transactions, block.Transactions)

	var td *big.Int
	if td, err = bc.bcStore.GetBlockTotalDifficulty(block.Header.PreviousBlockHash); err != nil {
		return err
	}

	blockIndex := NewBlockIndex(blockStatedb, currentBlock, td.Add(td, block.Header.Difficulty))

	isHead := bc.blockLeaves.IsBestBlockIndex(blockIndex)
	bc.blockLeaves.Add(blockIndex)
	bc.blockLeaves.RemoveByHash(block.Header.PreviousBlockHash)
	bc.headerChain.WriteHeader(currentBlock.Header)

	// If the new block has larger TD, the canonical chain will be changed.
	// In this case, need to update the height-to-blockHash mapping for the new canonical chain.
	if isHead {
		if err = bc.updateHashByHeight(block); err != nil {
			return err
		}
	}

	if err = bc.bcStore.PutBlock(block, td, isHead); err != nil {
		return err
	}

	// FIXME: write the block and update the account state in a batch.
	// Otherwise, restore the account state during service startup.
	if err = batch.Commit(); err != nil {
		return err
	}

	committed = true

	return nil
}

func (bc *Blockchain) validateBlock(block, preBlock *types.Block) error {
	if !block.HeaderHash.Equal(block.Header.Hash()) {
		return ErrBlockHashMismatch
	}

	txsHash := types.MerkleRootHash(block.Transactions)
	if !txsHash.Equal(block.Header.TxHash) {
		return ErrBlockTxsHashMismatch
	}

	if block.Header.Height != preBlock.Header.Height+1 {
		return ErrBlockInvalidHeight
	}

	return bc.engine.ValidateHeader(block.Header)
}

// GetStore returns the blockchain store instance.
func (bc *Blockchain) GetStore() store.BlockchainStore {
	return bc.bcStore
}

// applyTxs processes the txs in the specified block and returns the new state DB of the block.
// This method supposes the specified block is validated.
func (bc *Blockchain) applyTxs(block, preBlock *types.Block) (*state.Statedb, error) {
	minerRewardTx, err := bc.validateMinerRewardTx(block)
	if err != nil {
		return nil, err
	}

	statedb, err := state.NewStatedb(preBlock.Header.StateHash, bc.accountStateDB)
	if err != nil {
		return nil, err
	}

	if err := updateStatedb(statedb, minerRewardTx, block.Transactions[1:]); err != nil {
		return nil, err
	}

	return statedb, nil
}

func (bc *Blockchain) validateMinerRewardTx(block *types.Block) (*types.Transaction, error) {
	if len(block.Transactions) == 0 {
		return nil, ErrBlockEmptyTxs
	}

	minerRewardTx := block.Transactions[0]
	if minerRewardTx.Data == nil || minerRewardTx.Data.To == nil {
		return nil, ErrBlockInvalidToAddress
	}

	if !bytes.Equal(minerRewardTx.Data.To.Bytes(), block.Header.Creator.Bytes()) {
		return nil, ErrBlockCoinbaseMismatch
	}

	if minerRewardTx.Data.Amount == nil {
		return nil, types.ErrAmountNil
	}

	if minerRewardTx.Data.Amount.Sign() < 0 {
		return nil, types.ErrAmountNegative
	}

	if err := bc.engine.ValidateRewardAmount(minerRewardTx.Data.Amount); err != nil {
		return nil, err
	}

	return minerRewardTx, nil
}

func updateStatedb(statedb *state.Statedb, minerRewardTx *types.Transaction, txs []*types.Transaction) error {
	// process miner reward
	stateObj := statedb.GetOrNewStateObject(*minerRewardTx.Data.To)
	stateObj.AddAmount(minerRewardTx.Data.Amount)

	// process other txs
	for _, tx := range txs {
		if err := tx.Validate(statedb); err != nil {
			return err
		}

		if tx.Data.To == nil {
			return errContractCreationNotSupported
		}

		fromStateObj := statedb.GetOrNewStateObject(tx.Data.From)
		fromStateObj.SubAmount(tx.Data.Amount)
		fromStateObj.SetNonce(tx.Data.AccountNonce + 1)

		toStateObj := statedb.GetOrNewStateObject(*tx.Data.To)
		toStateObj.AddAmount(tx.Data.Amount)
	}

	return nil
}

// updateHashByHeight updates the height-to-hash mapping for the specified new HEAD block in the canonical chain.
func (bc *Blockchain) updateHashByHeight(block *types.Block) error {
	// Delete height-to-hash mappings with the larger height than that of the new HEAD block in the canonical chain.
	for i := block.Header.Height + 1; ; i++ {
		deleted, err := bc.bcStore.DeleteBlockHash(i)
		if err != nil {
			return err
		}

		if !deleted {
			break
		}
	}

	// Overwrite stale canonical height-to-hash mappings
	for headerHash := block.Header.PreviousBlockHash; !headerHash.Equal(common.EmptyHash); {
		header, err := bc.bcStore.GetBlockHeader(headerHash)
		if err != nil {
			return err
		}

		canonicalHash, err := bc.bcStore.GetBlockHash(header.Height)
		if err != nil {
			return err
		}

		if headerHash.Equal(canonicalHash) {
			break
		}

		if err = bc.bcStore.PutBlockHash(header.Height, headerHash); err != nil {
			return err
		}

		headerHash = header.PreviousBlockHash
	}

	return nil
}
