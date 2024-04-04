/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package blocksprovider

import (
	"context"
	"math"
	"sync"
	"time"

	"github.com/hyperledger/fabric-lib-go/bccsp"
	"github.com/hyperledger/fabric-lib-go/common/flogging"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric-protos-go/gossip"
	"github.com/hyperledger/fabric-protos-go/orderer"
	"github.com/hyperledger/fabric/common/deliverclient/orderers"
	"github.com/hyperledger/fabric/internal/pkg/identity"
	"google.golang.org/grpc"
)

// LedgerInfo an adapter to provide the interface to query
// the ledger committer for current ledger height
//
//go:generate counterfeiter -o fake/ledger_info.go --fake-name LedgerInfo . LedgerInfo
type LedgerInfo interface {
	// LedgerHeight returns current local ledger height
	LedgerHeight() (uint64, error)

	// GetCurrentBlockHash returns the block header hash of the last block in the ledger.
	GetCurrentBlockHash() ([]byte, error)
}

// GossipServiceAdapter serves to provide basic functionality
// required from gossip service by delivery service
//
//go:generate counterfeiter -o fake/gossip_service_adapter.go --fake-name GossipServiceAdapter . GossipServiceAdapter
type GossipServiceAdapter interface {
	// AddPayload adds payload to the local state sync buffer
	AddPayload(chainID string, payload *gossip.Payload) error

	// Gossip the message across the peers
	Gossip(msg *gossip.GossipMessage)
}

//go:generate counterfeiter -o fake/orderer_connection_source_factory.go --fake-name OrdererConnectionSourceFactory . OrdererConnectionSourceFactory
type OrdererConnectionSourceFactory orderers.ConnectionSourceCreator

//go:generate counterfeiter -o fake/orderer_connection_source.go --fake-name OrdererConnectionSource . OrdererConnectionSource
type OrdererConnectionSource orderers.ConnectionSourcer

//go:generate counterfeiter -o fake/dialer.go --fake-name Dialer . Dialer
type Dialer interface {
	Dial(address string, rootCerts [][]byte) (*grpc.ClientConn, error)
}

//go:generate counterfeiter -o fake/deliver_streamer.go --fake-name DeliverStreamer . DeliverStreamer
type DeliverStreamer interface {
	Deliver(context.Context, *grpc.ClientConn) (orderer.AtomicBroadcast_DeliverClient, error)
}

// MaxRetryDurationExceededHandler is a function that decides what to do in case the total time the component spends in
// reconnection attempts is exceeded. If it returns true, it means that the component should stop retrying.
//
// In the peer, with gossip and a dynamic leader, stopping causes the gossip leader to yield.
// In the peer, with gossip and a static leader, we never stop.
type MaxRetryDurationExceededHandler func() (stopRetries bool)

const backoffExponentBase = 1.2

// Deliverer the CFT implementation of the deliverservice.BlockDeliverer interface.
type Deliverer struct {
	ChannelID              string
	BlockHandler           BlockHandler
	Ledger                 LedgerInfo
	UpdatableBlockVerifier UpdatableBlockVerifier
	Dialer                 Dialer
	OrderersSourceFactory  OrdererConnectionSourceFactory
	CryptoProvider         bccsp.BCCSP
	DoneC                  chan struct{}
	Signer                 identity.SignerSerializer
	DeliverStreamer        DeliverStreamer
	Logger                 *flogging.FabricLogger

	// The maximal value of the actual retry interval, which cannot increase beyond this value
	MaxRetryInterval time.Duration
	// The initial value of the actual retry interval, which is increased on every failed retry
	InitialRetryInterval time.Duration
	// After this duration, the MaxRetryDurationExceededHandler is called to decide whether to keep trying
	MaxRetryDuration time.Duration
	// This function is called after MaxRetryDuration of failed retries to decide whether to keep trying
	MaxRetryDurationExceededHandler MaxRetryDurationExceededHandler

	// TLSCertHash should be nil when TLS is not enabled
	TLSCertHash []byte // util.ComputeSHA256(b.credSupport.GetClientCertificate().Certificate[0])

	sleeper sleeper

	requester *DeliveryRequester
	orderers  OrdererConnectionSource

	mutex         sync.Mutex
	stopFlag      bool
	blockReceiver *BlockReceiver
}

func (d *Deliverer) Initialize(channelConfig *cb.Config) {
	d.requester = NewDeliveryRequester(
		d.ChannelID,
		d.Signer,
		d.TLSCertHash,
		d.Dialer,
		d.DeliverStreamer,
	)

	osLogger := flogging.MustGetLogger("peer.orderers")
	ordererSource := d.OrderersSourceFactory.CreateConnectionSource(osLogger, "")
	globalAddresses, orgAddresses, err := extractAddresses(d.ChannelID, channelConfig, d.CryptoProvider)
	if err != nil {
		// The bundle was created prior to calling this function, so it should not fail when we recreate it here.
		d.Logger.Panicf("Bundle creation should not have failed: %s", err)
	}
	ordererSource.Update(globalAddresses, orgAddresses)
	d.orderers = ordererSource
}

// DeliverBlocks used to pull out blocks from the ordering service to distribute them across peers
func (d *Deliverer) DeliverBlocks() {
	failureCounter := 0
	totalDuration := time.Duration(0)

	// InitialRetryInterval * backoffExponentBase^n > MaxRetryInterval
	// backoffExponentBase^n > MaxRetryInterval / InitialRetryInterval
	// n * log(backoffExponentBase) > log(MaxRetryInterval / InitialRetryInterval)
	// n > log(MaxRetryInterval / InitialRetryInterval) / log(backoffExponentBase)
	maxFailures := int(math.Log(float64(d.MaxRetryInterval)/float64(d.InitialRetryInterval)) / math.Log(backoffExponentBase))
	for {
		select {
		case <-d.DoneC:
			return
		default:
		}

		if failureCounter > 0 {
			var sleepDuration time.Duration
			if failureCounter-1 > maxFailures {
				sleepDuration = d.MaxRetryInterval // configured from peer.deliveryclient.reConnectBackoffThreshold
			} else {
				sleepDuration = time.Duration(math.Pow(backoffExponentBase, float64(failureCounter-1))*100) * time.Millisecond
			}
			totalDuration += sleepDuration
			if totalDuration > d.MaxRetryDuration {
				if d.MaxRetryDurationExceededHandler() {
					d.Logger.Warningf("attempted to retry block delivery for more than peer.deliveryclient.reconnectTotalTimeThreshold duration (%s), giving up", d.MaxRetryDuration)
					return
				}
				d.Logger.Warningf("peer is a static leader, ignoring peer.deliveryclient.reconnectTotalTimeThreshold")
			}
			d.Logger.Warningf("Disconnected from ordering service. Attempt to re-connect in %v, totalDuration is %v", sleepDuration, totalDuration)
			d.sleeper.Sleep(sleepDuration, d.DoneC)
		}

		ledgerHeight, err := d.Ledger.LedgerHeight()
		if err != nil {
			d.Logger.Error("Did not return ledger height, something is critically wrong", err)
			return
		}

		endpoint, err := d.orderers.RandomEndpoint()
		if err != nil {
			d.Logger.Warningf("Could not connect to ordering service: could not get orderer endpoints: %s", err)
			failureCounter++
			continue
		}

		seekInfoEnv, err := d.requester.SeekInfoBlocksFrom(ledgerHeight)
		if err != nil {
			d.Logger.Error("Could not create a signed Deliver SeekInfo message, something is critically wrong", err)
			return
		}

		deliverClient, cancel, err := d.requester.Connect(seekInfoEnv, endpoint)
		if err != nil {
			d.Logger.Warningf("Could not connect to ordering service: %s", err)
			failureCounter++
			continue
		}

		d.mutex.Lock()
		blockReceiver := &BlockReceiver{
			channelID:              d.ChannelID,
			blockHandler:           d.BlockHandler,
			updatableBlockVerifier: d.UpdatableBlockVerifier,
			deliverClient:          deliverClient,
			cancelSendFunc:         cancel,
			recvC:                  make(chan *orderer.DeliverResponse),
			stopC:                  make(chan struct{}),
			endpoint:               endpoint,
			logger:                 d.Logger.With("orderer-address", endpoint.Address),
		}
		d.blockReceiver = blockReceiver
		d.mutex.Unlock()

		blockReceiver.Start() // starts an internal goroutine
		onSuccess := func(blockNum uint64, channelConfig *cb.Config) {
			failureCounter = 0
			totalDuration = time.Duration(0)

			if channelConfig != nil {
				globalAddresses, orgAddresses, err := extractAddresses(d.ChannelID, channelConfig, d.CryptoProvider)
				if err != nil {
					// The bundle was created prior to calling this function, so it should not fail when we recreate it here.
					d.Logger.Panicf("Bundle creation should not have failed: %s", err)
				}
				d.orderers.Update(globalAddresses, orgAddresses)
			}
		}
		if err := blockReceiver.ProcessIncoming(onSuccess); err != nil {
			switch err.(type) {
			case *errRefreshEndpoint:
				// Don't count it as an error, we'll reconnect immediately.
			case *ErrStopping:
				// Don't count it as an error, it is a signal to stop.
			default:
				d.Logger.Warningf("Failure in processing incoming messages: %s", err)
				failureCounter++
			}
		}
	}
}

// Stop stops blocks delivery provider
func (d *Deliverer) Stop() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.stopFlag {
		d.Logger.Debugf("Deliverer already stopped")
		return
	}

	d.stopFlag = true
	close(d.DoneC)
	d.blockReceiver.Stop()
	d.Logger.Info("Deliverer stopped")
}

func (d *Deliverer) setSleeperFunc(sleepFunc func(duration time.Duration)) {
	d.sleeper.sleep = sleepFunc
}
