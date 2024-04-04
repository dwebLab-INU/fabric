/*
Copyright IBM Corp. 2016 All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package txvalidator

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"reflect"
	"strconv"
	"time"

	fountain "github.com/Watchdog-Network/gofountain"
	types "github.com/Watchdog-Network/types"
	consumer "github.com/hyperledger/fabric/weaveutils/consumer"
	padding "github.com/hyperledger/fabric/weaveutils/equalization"
	producer "github.com/hyperledger/fabric/weaveutils/producer"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/lovoo/goka"

	"github.com/golang/protobuf/proto"
	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	"github.com/hyperledger/fabric-protos-go/common"
	mspprotos "github.com/hyperledger/fabric-protos-go/msp"
	"github.com/hyperledger/fabric-protos-go/peer"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/common/configtx"
	commonerrors "github.com/hyperledger/fabric/common/errors"
	"github.com/hyperledger/fabric/common/policies"
	"github.com/hyperledger/fabric/core/committer/txvalidator/plugin"
	"github.com/hyperledger/fabric/core/committer/txvalidator/v20/plugindispatcher"
	"github.com/hyperledger/fabric/core/common/validation"
	"github.com/hyperledger/fabric/core/ledger"
	"github.com/hyperledger/fabric/internal/pkg/txflags"
	"github.com/hyperledger/fabric/msp"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

// // This codec allows marshalling (encode) and unmarshalling (decode) the user to and from the
// // goka group table.
type ackCodec struct{}

type CurrentBlock struct {
	Id []byte

	// ackResultChan indicates that the producer keep sending encoded symbols
	// true = stop, false = push
	ackResultChan chan bool

	produceChan chan kafka.Event
}

var (
	kafkabroker1 = os.Getenv("KAFKA_BROKER1")
	kafkabroker2 = os.Getenv("KAFKA_BROKER2")
	kafkabroker3 = os.Getenv("KAFKA_BROKER3")

	tmc *goka.TopicManagerConfig

	ackResultChan = make(chan bool)

	kafkaProducer *kafka.Producer
	kafkaConsumer *kafka.Consumer
)

// init sets goka default settings
func init() {
	tmc = goka.NewTopicManagerConfig()
	tmc.Table.Replication = 3
	tmc.Stream.Replication = 3

	kafkaProducer = producer.Config()
	kafkaConsumer = consumer.Config()
}

func (cb *CurrentBlock) setAckChannel(val bool) {
	cb.ackResultChan <- val
}

// Encodes types.Ack into []byte
func (ac *ackCodec) Encode(value interface{}) ([]byte, error) {
	if _, isAck := value.(*types.ACK); !isAck {
		return nil, fmt.Errorf("ACK Codec requires value *types.ACK, got %T", value)
	}

	return json.Marshal(value)
}

// Decodes a types.Ack from []byte to it's go representation
func (ac *ackCodec) Decode(data []byte) (interface{}, error) {
	var c types.ACK
	err := json.Unmarshal(data, &c)
	if err != nil {
		return nil, fmt.Errorf("ackCodec Decode Error: %v", err)
	}
	return &c, nil
}

// Semaphore provides to the validator means for synchronisation
type Semaphore interface {
	// Acquire implements semaphore-like acquire semantics
	Acquire(ctx context.Context) error

	// Release implements semaphore-like release semantics
	Release()
}

// ChannelResources provides access to channel artefacts or
// functions to interact with them
type ChannelResources interface {
	// MSPManager returns the MSP manager for this channel
	MSPManager() msp.MSPManager

	// Apply attempts to apply a configtx to become the new config
	Apply(configtx *common.ConfigEnvelope) error

	// GetMSPIDs returns the IDs for the application MSPs
	// that have been defined in the channel
	GetMSPIDs() []string

	// Capabilities defines the capabilities for the application portion of this channel
	Capabilities() channelconfig.ApplicationCapabilities
}

// LedgerResources provides access to ledger artefacts or
// functions to interact with them
type LedgerResources interface {
	// TxIDExists returns true if the specified txID is already present in one of the already committed blocks
	TxIDExists(txID string) (bool, error)

	// NewQueryExecutor gives handle to a query executor.
	// A client can obtain more than one 'QueryExecutor's for parallel execution.
	// Any synchronization should be performed at the implementation level if required
	NewQueryExecutor() (ledger.QueryExecutor, error)
}

// Dispatcher is an interface to decouple tx validator
// and plugin dispatcher
type Dispatcher interface {
	// Dispatch invokes the appropriate validation plugin for the supplied transaction in the block
	Dispatch(seq int, payload *common.Payload, envBytes []byte, block *common.Block) (peer.TxValidationCode, error)
}

//go:generate mockery -dir . -name ChannelResources -case underscore -output mocks/
//go:generate mockery -dir . -name LedgerResources -case underscore -output mocks/
//go:generate mockery -dir . -name Dispatcher -case underscore -output mocks/

//go:generate mockery -dir . -name QueryExecutor -case underscore -output mocks/

// QueryExecutor is the local interface that used to generate mocks for foreign interface.
type QueryExecutor interface {
	ledger.QueryExecutor
}

//go:generate mockery -dir . -name ChannelPolicyManagerGetter -case underscore -output mocks/

// ChannelPolicyManagerGetter is the local interface that used to generate mocks for foreign interface.
type ChannelPolicyManagerGetter interface {
	policies.ChannelPolicyManagerGetter
}

//go:generate mockery -dir . -name PolicyManager -case underscore -output mocks/

type PolicyManager interface {
	policies.Manager
}

//go:generate mockery -dir plugindispatcher/ -name CollectionResources -case underscore -output mocks/

// TxValidator is the implementation of Validator interface, keeps
// reference to the ledger to enable tx simulation
// and execution of plugins
type TxValidator struct {
	ChannelID        string
	Semaphore        Semaphore
	ChannelResources ChannelResources
	LedgerResources  LedgerResources
	Dispatcher       Dispatcher
	CryptoProvider   bccsp.BCCSP
}

var logger = flogging.MustGetLogger("committer.txvalidator")

type blockValidationRequest struct {
	block *common.Block
	d     []byte
	tIdx  int
}

type blockValidationResult struct {
	tIdx           int
	validationCode peer.TxValidationCode
	err            error
	txid           string
}

// NewTxValidator creates new transactions validator
func NewTxValidator(
	channelID string,
	sem Semaphore,
	cr ChannelResources,
	ler LedgerResources,
	lcr plugindispatcher.LifecycleResources,
	cor plugindispatcher.CollectionResources,
	pm plugin.Mapper,
	channelPolicyManagerGetter policies.ChannelPolicyManagerGetter,
	cryptoProvider bccsp.BCCSP,
) *TxValidator {
	// Encapsulates interface implementation
	pluginValidator := plugindispatcher.NewPluginValidator(pm, ler, &dynamicDeserializer{cr: cr}, &dynamicCapabilities{cr: cr}, channelPolicyManagerGetter, cor)
	return &TxValidator{
		ChannelID:        channelID,
		Semaphore:        sem,
		ChannelResources: cr,
		LedgerResources:  ler,
		Dispatcher:       plugindispatcher.New(channelID, cr, ler, lcr, pluginValidator),
		CryptoProvider:   cryptoProvider,
	}
}

func (v *TxValidator) chainExists(chain string) bool {
	// TODO: implement this function!
	return true
}

// Validate performs the validation of a block. The validation
// of each transaction in the block is performed in parallel.
// The approach is as follows: the committer thread starts the
// tx validation function in a goroutine (using a semaphore to cap
// the number of concurrent validating goroutines). The committer
// thread then reads results of validation (in orderer of completion
// of the goroutines) from the results channel. The goroutines
// perform the validation of the txs in the block and enqueue the
// validation result in the results channel. A few note-worthy facts:
//  1. to keep the approach simple, the committer thread enqueues
//     all transactions in the block and then moves on to reading the
//     results.
//  2. for parallel validation to work, it is important that the
//     validation function does not change the state of the system.
//     Otherwise the order in which validation is perform matters
//     and we have to resort to sequential validation (or some locking).
//     This is currently true, because the only function that affects
//     state is when a config transaction is received, but they are
//     guaranteed to be alone in the block. If/when this assumption
//     is violated, this code must be changed.
func (v *TxValidator) Validate(block *common.Block) error {
	var err error
	var errPos int

	cb := CurrentBlock{
		Id:            block.Header.DataHash,
		ackResultChan: make(chan bool),
		produceChan:   make(chan kafka.Event),
	}

	// run background when it receives first block
	if block.Header.Number <= 1 {
		go cb.ackListener()
	}

	startValidation := time.Now() // timer to log Validate block duration
	logger.Debugf("[%s] START Block Validation for block [%d]", v.ChannelID, block.Header.Number)

	// Initialize trans as valid here, then set invalidation reason code upon invalidation below
	txsfltr := txflags.New(len(block.Data.Data))
	// array of txids
	txidArray := make([]string, len(block.Data.Data))

	results := make(chan *blockValidationResult)
	go func() {
		for tIdx, d := range block.Data.Data {
			// ensure that we don't have too many concurrent validation workers
			v.Semaphore.Acquire(context.Background())

			go func(index int, data []byte) {
				defer v.Semaphore.Release()

				v.validateTx(&blockValidationRequest{
					d:     data,
					block: block,
					tIdx:  index,
				}, results)
			}(tIdx, d)
		}
	}()

	logger.Debugf("expecting %d block validation responses", len(block.Data.Data))

	// now we read responses in the order in which they come back
	for i := 0; i < len(block.Data.Data); i++ {
		res := <-results

		if res.err != nil {
			// if there is an error, we buffer its value, wait for
			// all workers to complete validation and then return
			// the error from the first tx in this block that returned an error
			logger.Debugf("got terminal error %s for idx %d", res.err, res.tIdx)

			if err == nil || res.tIdx < errPos {
				err = res.err
				errPos = res.tIdx
			}
		} else {
			// if there was no error, we set the txsfltr and we set the
			// txsChaincodeNames and txsUpgradedChaincodes maps
			logger.Debugf("got result for idx %d, code %d", res.tIdx, res.validationCode)

			txsfltr.SetFlag(res.tIdx, res.validationCode)

			if res.validationCode == peer.TxValidationCode_VALID {
				txidArray[res.tIdx] = res.txid
			}
		}
	}

	// if we're here, all workers have completed the validation.
	// If there was an error we return the error from the first
	// tx in this block that returned an error
	if err != nil {
		return err
	}

	// we mark invalid any transaction that has a txid
	// which is equal to that of a previous tx in this block
	markTXIdDuplicates(txidArray, txsfltr)

	// make sure no transaction has skipped validation
	err = v.allValidated(txsfltr, block)
	if err != nil {
		return err
	}

	// Initialize metadata structure
	protoutil.InitBlockMetadata(block)

	block.Metadata.Metadata[common.BlockMetadataIndex_TRANSACTIONS_FILTER] = txsfltr

	elapsedValidation := time.Since(startValidation) / time.Millisecond // duration in ms
	logger.Infof("[%s] Validated block [%d] in %dms", v.ChannelID, block.Header.Number, elapsedValidation)

	cb.produceContribution(block, v.ChannelID)

	cb.producer(block, v.ChannelID)

	return nil
}

// produceContribution monitors the number of blocks sent by the producer to detect which producers are not working.
func (cb *CurrentBlock) produceContribution(block *common.Block, channelID string) {
	topic := "contribution-topic"

	deliveryChan := make(chan kafka.Event, 1000000)

	data := types.FabricChannel{
		Type:         1,
		ChannelId:    channelID,
		BlockNumber:  int(block.Header.Number),
		Transactions: len(block.GetData().GetData()),
	}

	value, _ := json.Marshal(data)

	err := kafkaProducer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          value,
	}, deliveryChan)
	if err != nil {
		panic(err)
	}
}

// producer structures the block data.
func (cb *CurrentBlock) producer(block *common.Block, channelID string) {
	host := os.Getenv("CORE_PEER_ID")

	parsedBlock := types.HLFEncodedBlock{
		BlockHeader:   types.BlockHeader{Number: block.Header.Number, PreviousHash: block.Header.PreviousHash, DataHash: block.Header.DataHash},
		BlockData:     types.BlockData{Data: block.Data.Data},
		BlockMetadata: types.BlockMetadata{Metadata: block.Metadata.Metadata},
	}

	cb.Id = parsedBlock.BlockHeader.DataHash

	cb.produceSymbols(host, channelID, parsedBlock)
}

// produceSymbols encodes blockdata ans divides it into symbols.
// The divided symbols are bundled and delivered to kafka
// to balance the time write symbols on the kafka and the time due to network delay.
func (cb *CurrentBlock) produceSymbols(peer string, channelID string, parsedBlock types.HLFEncodedBlock) {

	peerNumber, _ := strconv.Atoi(os.Getenv("PEER_NUMBER"))

	// numSourceSymbols means how many source symbols the input message will be divided into
	// and the minimum number of source symbols required for decoding.
	numSourceSymbols, _ := strconv.Atoi(os.Getenv("NUM_SOURCE_SYMBOLS"))

	// symbolAlignmentSize, the size of ach symbol in the source message in bytes.
	symbolAlignmentSize, _ := strconv.Atoi(os.Getenv("SYMBOL_ALIGNMENT_SIZE"))

	// numEncodedSourceSymbols means how many encoded source symbols will be created using source symbols
	// and the maximum number of source symbols required for decoding.
	numEncodedSourceSymbols, _ := strconv.Atoi(os.Getenv("NUM_ENCODED_SOURCE_SYMBOLS"))

	marshalledBlock := padding.EqualizeParsedBlockLengths(numSourceSymbols, symbolAlignmentSize, numEncodedSourceSymbols, parsedBlock)

	encodingData := fountain.Encode(marshalledBlock, numSourceSymbols, symbolAlignmentSize, numEncodedSourceSymbols)
	hash := sha256.Sum256(marshalledBlock)

	var sbdata types.SymbolBundleData

	bundle, _ := strconv.Atoi(os.Getenv("BUNDLE"))
	var count = 0
	for i := 0; i < len(encodingData)/peerNumber; i++ {
		// Push an encoded symbol until receiving OK sign
		select {
		case stop := <-cb.ackResultChan:
			if stop {
				cb.setAckChannel(false)
				return
			}
		default:
			var sdata types.SymbolData
			sdata.Id = peer
			sdata.SourceData = encodingData[i]
			sdata.Length = len(marshalledBlock)
			sdata.Hash = hash
			sdata.NumSourceSymbols = numSourceSymbols
			sdata.SymbolAlignmentSize = symbolAlignmentSize
			sdata.NumEncodedSourceSymbols = numEncodedSourceSymbols

			sbdata.Symbols = append(sbdata.Symbols, sdata)

			if (i+1)%bundle == 0 {
				sbdata.Channel = channelID
				sbdata.Id = peer
				sbdata.Hash = hash
				sbdata.Length = len(marshalledBlock)
				sbdata.NumSourceSymbols = numSourceSymbols
				sbdata.SymbolAlignmentSize = symbolAlignmentSize
				sbdata.NumEncodedSourceSymbols = numEncodedSourceSymbols

				value, err := json.Marshal(&sbdata)
				if err != nil {
					log.Fatal(err)
				}

				sbdata.Symbols = sbdata.Symbols[:0]

				cb.pushBundle(channelID, value)
				count++
			}
		}
	}
}

// pushBundle sends an encoding symbol to the kafka partition (i.e., topic).
func (cb *CurrentBlock) pushBundle(channelID string, value []byte) {
	deliveryChan := make(chan kafka.Event, 10000)

	go func() {
		select {
		case deliveryReport := <-deliveryChan:
			m := deliveryReport.(*kafka.Message)

			if m.TopicPartition.Error != nil {
				fmt.Printf("Delivery failed: %v\n", m.TopicPartition.Error)
			} else if string(m.Value) == "Over rate limit" {
				fmt.Println("Over rate limit")
				time.Sleep(5 * time.Second)
			} else {
				fmt.Printf("[%s] => Delivered message to topic [%d] at offset %v with key '%s' \n", *m.TopicPartition.Topic, m.TopicPartition.Partition, m.TopicPartition.Offset, m.Key)
			}
		}
	}()

	// Push message to Kafka with key by specifying topic
	err := kafkaProducer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &channelID, Partition: kafka.PartitionAny},
		Value:          value,
	}, deliveryChan) // cb.produceChan is for checking acks
	if err != nil {
		fmt.Printf("Failed to produce message: %s\n", err)
		return
	}
}

// ackListener waits for ack message from real-time processor that indicates the processor
// successfully decoded the block data.
// When a producer pulls OK sign, it immediately stop to push encoded symbols into Kafka.
func (cb *CurrentBlock) ackListener() {
	fmt.Println("Now listening to Kafka ACK ...")

	kafkaConsumer.SubscribeTopics([]string{"ack-topic"}, nil)
	defer kafkaConsumer.Close()

	for {
		msg, err := kafkaConsumer.ReadMessage(-1)
		if err == nil {
			recvAck := types.ACK{}
			json.Unmarshal(msg.Value, &recvAck)

			if reflect.DeepEqual(recvAck.Id, cb.Id) {
				cb.setAckChannel(true)
				fmt.Println("Received msg:", string(recvAck.AckMessage), "for ID", cb.Id)
			} else {
				fmt.Println("ID does not match, Received ID:", recvAck.Id, "Current ID:", cb.Id)
			}
		}
	}
}

// allValidated returns error if some of the validation flags have not been set
// during validation
func (v *TxValidator) allValidated(txsfltr txflags.ValidationFlags, block *common.Block) error {
	for id, f := range txsfltr {
		if peer.TxValidationCode(f) == peer.TxValidationCode_NOT_VALIDATED {
			return errors.Errorf("transaction %d in block %d has skipped validation", id, block.Header.Number)
		}
	}

	return nil
}

func markTXIdDuplicates(txids []string, txsfltr txflags.ValidationFlags) {
	txidMap := make(map[string]struct{})

	for id, txid := range txids {
		if txid == "" {
			continue
		}

		_, in := txidMap[txid]
		if in {
			logger.Error("Duplicate txid", txid, "found, skipping")
			txsfltr.SetFlag(id, peer.TxValidationCode_DUPLICATE_TXID)
		} else {
			txidMap[txid] = struct{}{}
		}
	}
}

func (v *TxValidator) validateTx(req *blockValidationRequest, results chan<- *blockValidationResult) {
	block := req.block
	d := req.d
	tIdx := req.tIdx
	txID := ""

	if d == nil {
		results <- &blockValidationResult{
			tIdx: tIdx,
		}
		return
	}

	if env, err := protoutil.GetEnvelopeFromBlock(d); err != nil {
		logger.Warningf("Error getting tx from block: %+v", err)
		results <- &blockValidationResult{
			tIdx:           tIdx,
			validationCode: peer.TxValidationCode_INVALID_OTHER_REASON,
		}
		return
	} else if env != nil {
		// validate the transaction: here we check that the transaction
		// is properly formed, properly signed and that the security
		// chain binding proposal to endorsements to tx holds. We do
		// NOT check the validity of endorsements, though. That's a
		// job for the validation plugins
		logger.Debugf("[%s] validateTx starts for block %p env %p txn %d", v.ChannelID, block, env, tIdx)
		defer logger.Debugf("[%s] validateTx completes for block %p env %p txn %d", v.ChannelID, block, env, tIdx)
		var payload *common.Payload
		var err error
		var txResult peer.TxValidationCode

		if payload, txResult = validation.ValidateTransaction(env, v.CryptoProvider); txResult != peer.TxValidationCode_VALID {
			logger.Errorf("Invalid transaction with index %d", tIdx)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: txResult,
			}
			return
		}

		chdr, err := protoutil.UnmarshalChannelHeader(payload.Header.ChannelHeader)
		if err != nil {
			logger.Warningf("Could not unmarshal channel header, err %s, skipping", err)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: peer.TxValidationCode_INVALID_OTHER_REASON,
			}
			return
		}

		channel := chdr.ChannelId
		logger.Debugf("Transaction is for channel %s", channel)

		if !v.chainExists(channel) {
			logger.Errorf("Dropping transaction for non-existent channel %s", channel)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: peer.TxValidationCode_TARGET_CHAIN_NOT_FOUND,
			}
			return
		}

		if common.HeaderType(chdr.Type) == common.HeaderType_ENDORSER_TRANSACTION {

			txID = chdr.TxId

			// Check duplicate transactions
			erroneousResultEntry := v.checkTxIdDupsLedger(tIdx, chdr, v.LedgerResources)
			if erroneousResultEntry != nil {
				results <- erroneousResultEntry
				return
			}

			// Validate tx with plugins
			logger.Debug("Validating transaction with plugins")
			cde, err := v.Dispatcher.Dispatch(tIdx, payload, d, block)
			if err != nil {
				logger.Errorf("Dispatch for transaction txId = %s returned error: %s", txID, err)
				switch err.(type) {
				case *commonerrors.VSCCExecutionFailureError:
					results <- &blockValidationResult{
						tIdx: tIdx,
						err:  err,
					}
					return
				case *commonerrors.VSCCInfoLookupFailureError:
					results <- &blockValidationResult{
						tIdx: tIdx,
						err:  err,
					}
					return
				default:
					results <- &blockValidationResult{
						tIdx:           tIdx,
						validationCode: cde,
					}
					return
				}
			}
		} else if common.HeaderType(chdr.Type) == common.HeaderType_CONFIG {
			configEnvelope, err := configtx.UnmarshalConfigEnvelope(payload.Data)
			if err != nil {
				err = errors.WithMessage(err, "error unmarshalling config which passed initial validity checks")
				logger.Criticalf("%+v", err)
				results <- &blockValidationResult{
					tIdx: tIdx,
					err:  err,
				}
				return
			}

			logger.Debugw("Config transaction envelope passed validation checks", "channel", channel)
			if err := v.ChannelResources.Apply(configEnvelope); err != nil {
				err = errors.WithMessage(err, "error validating config which passed initial validity checks")
				logger.Criticalf("%+v", err)
				results <- &blockValidationResult{
					tIdx: tIdx,
					err:  err,
				}
				return
			}
			logger.Infow("Config transaction validated and applied to channel resources", "channel", channel)
		} else {
			logger.Warningf("Unknown transaction type [%s] in block number [%d] transaction index [%d]",
				common.HeaderType(chdr.Type), block.Header.Number, tIdx)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: peer.TxValidationCode_UNKNOWN_TX_TYPE,
			}
			return
		}

		if _, err := proto.Marshal(env); err != nil {
			logger.Warningf("Cannot marshal transaction: %s", err)
			results <- &blockValidationResult{
				tIdx:           tIdx,
				validationCode: peer.TxValidationCode_MARSHAL_TX_ERROR,
			}
			return
		}
		// Succeeded to pass down here, transaction is valid
		results <- &blockValidationResult{
			tIdx:           tIdx,
			validationCode: peer.TxValidationCode_VALID,
			txid:           txID,
		}
		return
	} else {
		logger.Warning("Nil tx from block")
		results <- &blockValidationResult{
			tIdx:           tIdx,
			validationCode: peer.TxValidationCode_NIL_ENVELOPE,
		}
		return
	}
}

// CheckTxIdDupsLedger returns a vlockValidationResult enhanced with the respective
// error codes if and only if there is transaction with the same transaction identifier
// in the ledger or no decision can be made for whether such transaction exists;
// the function returns nil if it has ensured that there is no such duplicate, such
// that its consumer can proceed with the transaction processing
func (v *TxValidator) checkTxIdDupsLedger(tIdx int, chdr *common.ChannelHeader, ldgr LedgerResources) *blockValidationResult {
	// Retrieve the transaction identifier of the input header
	txID := chdr.TxId

	// Look for a transaction with the same identifier inside the ledger
	exists, err := ldgr.TxIDExists(txID)
	if err != nil {
		logger.Errorf("Ledger failure while attempting to detect duplicate status for txid %s: %s", txID, err)
		return &blockValidationResult{
			tIdx: tIdx,
			err:  err,
		}
	}
	if exists {
		logger.Error("Duplicate transaction found, ", txID, ", skipping")
		return &blockValidationResult{
			tIdx:           tIdx,
			validationCode: peer.TxValidationCode_DUPLICATE_TXID,
		}
	}
	return nil
}

type dynamicDeserializer struct {
	cr ChannelResources
}

func (ds *dynamicDeserializer) DeserializeIdentity(serializedIdentity []byte) (msp.Identity, error) {
	return ds.cr.MSPManager().DeserializeIdentity(serializedIdentity)
}

func (ds *dynamicDeserializer) IsWellFormed(identity *mspprotos.SerializedIdentity) error {
	return ds.cr.MSPManager().IsWellFormed(identity)
}

type dynamicCapabilities struct {
	cr ChannelResources
}

func (ds *dynamicCapabilities) ACLs() bool {
	return ds.cr.Capabilities().ACLs()
}

func (ds *dynamicCapabilities) CollectionUpgrade() bool {
	return ds.cr.Capabilities().CollectionUpgrade()
}

func (ds *dynamicCapabilities) ForbidDuplicateTXIdInBlock() bool {
	return ds.cr.Capabilities().ForbidDuplicateTXIdInBlock()
}

func (ds *dynamicCapabilities) KeyLevelEndorsement() bool {
	return ds.cr.Capabilities().KeyLevelEndorsement()
}

func (ds *dynamicCapabilities) MetadataLifecycle() bool {
	// This capability no longer exists and should not be referenced in validation anyway
	return false
}

func (ds *dynamicCapabilities) PrivateChannelData() bool {
	return ds.cr.Capabilities().PrivateChannelData()
}

func (ds *dynamicCapabilities) StorePvtDataOfInvalidTx() bool {
	return ds.cr.Capabilities().StorePvtDataOfInvalidTx()
}

func (ds *dynamicCapabilities) Supported() error {
	return ds.cr.Capabilities().Supported()
}

func (ds *dynamicCapabilities) V1_1Validation() bool {
	return ds.cr.Capabilities().V1_1Validation()
}

func (ds *dynamicCapabilities) V1_2Validation() bool {
	return ds.cr.Capabilities().V1_2Validation()
}

func (ds *dynamicCapabilities) V1_3Validation() bool {
	return ds.cr.Capabilities().V1_3Validation()
}

func (ds *dynamicCapabilities) V2_0Validation() bool {
	return ds.cr.Capabilities().V2_0Validation()
}
