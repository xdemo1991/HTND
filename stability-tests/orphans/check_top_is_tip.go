package main

import (
	"github.com/Hoosat-Oy/htnd/domain/consensus/model/externalapi"
	"github.com/Hoosat-Oy/htnd/domain/consensus/utils/consensushashing"
	"github.com/Hoosat-Oy/htnd/stability-tests/common/rpc"
	"github.com/pkg/errors"
)

func checkTopBlockIsTip(rpcClient *rpc.Client, topBlock *externalapi.DomainBlock) error {
	selectedTipHashResponse, err := rpcClient.GetSelectedTipHash()
	if err != nil {
		return err
	}

	topBlockHash := consensushashing.BlockHash(topBlock)
	if selectedTipHashResponse.SelectedTipHash != topBlockHash.String() {
		return errors.Errorf("selectedTipHash is '%s' while expected to be topBlock's hash `%s`",
			selectedTipHashResponse.SelectedTipHash, topBlockHash)
	}

	return nil
}
