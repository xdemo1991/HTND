package server

import (
	"testing"

	"github.com/Hoosat-Oy/htnd/cmd/htnwallet/libhtnwallet/serialization"

	"github.com/Hoosat-Oy/htnd/cmd/htnwallet/keys"
	"github.com/Hoosat-Oy/htnd/util/txmass"

	"github.com/Hoosat-Oy/htnd/domain/dagconfig"

	"github.com/Hoosat-Oy/htnd/domain/consensus/model/externalapi"
	"github.com/Hoosat-Oy/htnd/domain/consensus/utils/consensushashing"
	"github.com/Hoosat-Oy/htnd/domain/consensus/utils/txscript"
	"github.com/Hoosat-Oy/htnd/domain/consensus/utils/utxo"

	"github.com/Hoosat-Oy/htnd/cmd/htnwallet/libhtnwallet"
	"github.com/Hoosat-Oy/htnd/domain/consensus"
	"github.com/Hoosat-Oy/htnd/domain/consensus/utils/testutils"
)

func TestEstimateMassAfterSignatures(t *testing.T) {
	testutils.ForAllNets(t, true, func(t *testing.T, consensusConfig *consensus.Config) {
		unsignedTransactionBytes, mnemonics, params, teardown := testEstimateMassIncreaseForSignaturesSetUp(t, consensusConfig)
		defer teardown(false)

		serverInstance := &server{
			params:           params,
			keysFile:         &keys.File{MinimumSignatures: 2},
			shutdown:         make(chan struct{}),
			addressSet:       make(walletAddressSet),
			txMassCalculator: txmass.NewCalculator(params.MassPerTxByte, params.MassPerScriptPubKeyByte, params.MassPerSigOp),
		}

		unsignedTransaction, err := serialization.DeserializePartiallySignedTransaction(unsignedTransactionBytes)
		if err != nil {
			t.Fatalf("Error deserializing unsignedTransaction: %s", err)
		}

		estimatedMassAfterSignatures, err := serverInstance.estimateMassAfterSignatures(unsignedTransaction)
		if err != nil {
			t.Fatalf("Error from estimateMassAfterSignatures: %s", err)
		}

		signedTxStep1Bytes, err := libhtnwallet.Sign(params, mnemonics[:1], unsignedTransactionBytes, false)
		if err != nil {
			t.Fatalf("Sign: %+v", err)
		}

		signedTxStep2Bytes, err := libhtnwallet.Sign(params, mnemonics[1:2], signedTxStep1Bytes, false)
		if err != nil {
			t.Fatalf("Sign: %+v", err)
		}

		extractedSignedTx, err := libhtnwallet.ExtractTransaction(signedTxStep2Bytes, false)
		if err != nil {
			t.Fatalf("ExtractTransaction: %+v", err)
		}

		actualMassAfterSignatures := serverInstance.txMassCalculator.CalculateTransactionMass(extractedSignedTx)

		if estimatedMassAfterSignatures != actualMassAfterSignatures {
			t.Errorf("Estimated mass after signatures: %d but actually got %d",
				estimatedMassAfterSignatures, actualMassAfterSignatures)
		}
	})
}

func testEstimateMassIncreaseForSignaturesSetUp(t *testing.T, consensusConfig *consensus.Config) (
	[]byte, []string, *dagconfig.Params, func(keepDataDir bool)) {

	consensusConfig.BlockCoinbaseMaturity = 0
	params := &consensusConfig.Params

	tc, teardown, err := consensus.NewFactory().NewTestConsensus(consensusConfig, "TestMultisig")
	if err != nil {
		t.Fatalf("Error setting up tc: %+v", err)
	}

	const numKeys = 3
	mnemonics := make([]string, numKeys)
	publicKeys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		var err error
		mnemonics[i], err = libhtnwallet.CreateMnemonic()
		if err != nil {
			t.Fatalf("CreateMnemonic: %+v", err)
		}

		publicKeys[i], err = libhtnwallet.MasterPublicKeyFromMnemonic(&consensusConfig.Params, mnemonics[i], true)
		if err != nil {
			t.Fatalf("MasterPublicKeyFromMnemonic: %+v", err)
		}
	}

	const minimumSignatures = 2
	path := "m/1/2/3"
	address, err := libhtnwallet.Address(params, publicKeys, minimumSignatures, path, false)
	if err != nil {
		t.Fatalf("Address: %+v", err)
	}

	scriptPublicKey, err := txscript.PayToAddrScript(address)
	if err != nil {
		t.Fatalf("PayToAddrScript: %+v", err)
	}

	coinbaseData := &externalapi.DomainCoinbaseData{
		ScriptPublicKey: scriptPublicKey,
		ExtraData:       nil,
	}

	fundingBlockHash, _, err := tc.AddBlock([]*externalapi.DomainHash{consensusConfig.GenesisHash}, coinbaseData, nil)
	if err != nil {
		t.Fatalf("AddBlock: %+v", err)
	}

	block1Hash, _, err := tc.AddBlock([]*externalapi.DomainHash{fundingBlockHash}, nil, nil)
	if err != nil {
		t.Fatalf("AddBlock: %+v", err)
	}

	block1, _, err := tc.GetBlock(block1Hash)
	if err != nil {
		t.Fatalf("GetBlock: %+v", err)
	}

	block1Tx := block1.Transactions[0]
	block1TxOut := block1Tx.Outputs[0]
	selectedUTXOs := []*libhtnwallet.UTXO{
		{
			Outpoint: &externalapi.DomainOutpoint{
				TransactionID: *consensushashing.TransactionID(block1.Transactions[0]),
				Index:         0,
			},
			UTXOEntry:      utxo.NewUTXOEntry(block1TxOut.Value, block1TxOut.ScriptPublicKey, true, 0),
			DerivationPath: path,
		},
	}

	unsignedTransaction, err := libhtnwallet.CreateUnsignedTransaction(publicKeys, minimumSignatures,
		[]*libhtnwallet.Payment{{
			Address: address,
			Amount:  10,
		}}, selectedUTXOs)
	if err != nil {
		t.Fatalf("CreateUnsignedTransactions: %+v", err)
	}

	return unsignedTransaction, mnemonics, params, teardown
}
