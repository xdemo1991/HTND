package rpc

import (
	"bytes"
	"encoding/hex"
	"github.com/daglabs/btcd/blockdag"
	"github.com/daglabs/btcd/btcjson"
	"github.com/daglabs/btcd/database"
	"github.com/daglabs/btcd/util"
	"github.com/daglabs/btcd/util/daghash"
	"github.com/daglabs/btcd/wire"
)

// handleGetRawTransaction implements the getRawTransaction command.
func handleGetRawTransaction(s *Server, cmd interface{}, closeChan <-chan struct{}) (interface{}, error) {
	c := cmd.(*btcjson.GetRawTransactionCmd)

	// Convert the provided transaction hash hex to a Hash.
	txID, err := daghash.NewTxIDFromStr(c.TxID)
	if err != nil {
		return nil, rpcDecodeHexError(c.TxID)
	}

	verbose := false
	if c.Verbose != nil {
		verbose = *c.Verbose != 0
	}

	// Try to fetch the transaction from the memory pool and if that fails,
	// try the block database.
	var msgTx *wire.MsgTx
	var blkHash *daghash.Hash
	isInMempool := false
	mempoolTx, err := s.cfg.TxMemPool.FetchTransaction(txID)
	if err != nil {
		if s.cfg.TxIndex == nil {
			return nil, &btcjson.RPCError{
				Code: btcjson.ErrRPCNoTxInfo,
				Message: "The transaction index must be " +
					"enabled to query the blockchain " +
					"(specify --txindex)",
			}
		}

		// Look up the location of the transaction.
		blockRegion, err := s.cfg.TxIndex.TxFirstBlockRegion(txID)
		if err != nil {
			context := "Failed to retrieve transaction location"
			return nil, internalRPCError(err.Error(), context)
		}
		if blockRegion == nil {
			return nil, rpcNoTxInfoError(txID)
		}

		// Load the raw transaction bytes from the database.
		var txBytes []byte
		err = s.cfg.DB.View(func(dbTx database.Tx) error {
			var err error
			txBytes, err = dbTx.FetchBlockRegion(blockRegion)
			return err
		})
		if err != nil {
			return nil, rpcNoTxInfoError(txID)
		}

		// When the verbose flag isn't set, simply return the serialized
		// transaction as a hex-encoded string.  This is done here to
		// avoid deserializing it only to reserialize it again later.
		if !verbose {
			return hex.EncodeToString(txBytes), nil
		}

		// Grab the block hash.
		blkHash = blockRegion.Hash

		// Deserialize the transaction
		var mtx wire.MsgTx
		err = mtx.Deserialize(bytes.NewReader(txBytes))
		if err != nil {
			context := "Failed to deserialize transaction"
			return nil, internalRPCError(err.Error(), context)
		}
		msgTx = &mtx
	} else {
		// When the verbose flag isn't set, simply return the
		// network-serialized transaction as a hex-encoded string.
		if !verbose {
			// Note that this is intentionally not directly
			// returning because the first return value is a
			// string and it would result in returning an empty
			// string to the client instead of nothing (nil) in the
			// case of an error.
			mtxHex, err := messageToHex(mempoolTx.MsgTx())
			if err != nil {
				return nil, err
			}
			return mtxHex, nil
		}

		msgTx = mempoolTx.MsgTx()
		isInMempool = true
	}

	// The verbose flag is set, so generate the JSON object and return it.
	var blkHeader *wire.BlockHeader
	var blkHashStr string
	if blkHash != nil {
		// Fetch the header from chain.
		header, err := s.cfg.DAG.HeaderByHash(blkHash)
		if err != nil {
			context := "Failed to fetch block header"
			return nil, internalRPCError(err.Error(), context)
		}

		blkHeader = header
		blkHashStr = blkHash.String()
	}

	confirmations, err := txConfirmations(s, msgTx.TxID())
	if err != nil {
		return nil, err
	}
	txMass, err := blockdag.CalcTxMass(util.NewTx(msgTx), s.cfg.DAG.UTXOSet())
	if err != nil {
		return nil, err
	}
	rawTxn, err := createTxRawResult(s.cfg.DAGParams, msgTx, txID.String(),
		blkHeader, blkHashStr, nil, &confirmations, isInMempool, txMass)
	if err != nil {
		return nil, err
	}
	return *rawTxn, nil
}