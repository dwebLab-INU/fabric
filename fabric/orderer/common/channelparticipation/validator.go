/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package channelparticipation

import (
	"bytes"

	"github.com/hyperledger/fabric-lib-go/bccsp/factory"
	cb "github.com/hyperledger/fabric-protos-go/common"
	"github.com/hyperledger/fabric/common/channelconfig"
	"github.com/hyperledger/fabric/orderer/common/types"
	"github.com/hyperledger/fabric/protoutil"
	"github.com/pkg/errors"
)

// ValidateJoinBlock checks whether this block can be used as a join block for the channel participation API.
// It returns the channel ID.
// It verifies that it is not a system channel by checking that consortiums config does not exist.
// It verifies it is an application channel by checking that the application group exists.
// It returns an error when it cannot be used as a join-block.
func ValidateJoinBlock(configBlock *cb.Block) (channelID string, err error) {
	if !protoutil.IsConfigBlock(configBlock) {
		return "", errors.New("block is not a config block")
	}

	if configBlock.Metadata == nil || len(configBlock.Metadata.Metadata) == 0 {
		return "", errors.New("invalid block: does not have metadata")
	}

	dataHash := protoutil.ComputeBlockDataHash(configBlock.Data)
	if !bytes.Equal(dataHash, configBlock.Header.DataHash) {
		return "", errors.New("invalid block: Header.DataHash is different from Hash(block.Data)")
	}

	envelope, err := protoutil.ExtractEnvelope(configBlock, 0)
	if err != nil {
		return "", err
	}

	cryptoProvider := factory.GetDefault()
	bundle, err := channelconfig.NewBundleFromEnvelope(envelope, cryptoProvider)
	if err != nil {
		return "", err
	}

	channelID = bundle.ConfigtxValidator().ChannelID()

	// Check channel type
	_, isSystemChannel := bundle.ConsortiumsConfig()
	if isSystemChannel {
		return "", errors.WithMessage(types.ErrSystemChannelNotSupported, "invalid config: contains consortiums")
	}

	_, isAppChannel := bundle.ApplicationConfig()
	if !isAppChannel {
		return "", errors.New("invalid config: must contain application config")
	}

	return channelID, err
}
