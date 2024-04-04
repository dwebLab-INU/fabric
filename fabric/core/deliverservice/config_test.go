/*
Copyright IBM Corp. All Rights Reserved.

SPDX-License-Identifier: Apache-2.0
*/

package deliverservice_test

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/hyperledger/fabric/core/deliverservice"
	"github.com/hyperledger/fabric/internal/pkg/comm"
	"github.com/spf13/viper"
)

func TestSecureOptsConfig(t *testing.T) {
	cwd, err := os.Getwd()
	require.NoError(t, err)
	certPath := filepath.Join(cwd, "testdata", "cert.pem")
	keyPath := filepath.Join(cwd, "testdata", "key.pem")

	certBytes, err := os.ReadFile(filepath.Join("testdata", "cert.pem"))
	require.NoError(t, err)

	keyBytes, err := os.ReadFile(filepath.Join("testdata", "key.pem"))
	require.NoError(t, err)

	t.Run("client specific cert", func(t *testing.T) {
		viper.Reset()
		defer viper.Reset()

		viper.Set("peer.tls.enabled", true)
		viper.Set("peer.tls.clientAuthRequired", true)
		viper.Set("peer.tls.clientCert.file", certPath)
		viper.Set("peer.tls.clientKey.file", keyPath)

		coreConfig := deliverservice.GlobalConfig()

		require.True(t, coreConfig.SecOpts.UseTLS)
		require.True(t, coreConfig.SecOpts.RequireClientCert)
		require.Equal(t, keyBytes, coreConfig.SecOpts.Key)
		require.Equal(t, certBytes, coreConfig.SecOpts.Certificate)
	})

	t.Run("fallback cert", func(t *testing.T) {
		viper.Reset()
		defer viper.Reset()

		viper.Set("peer.tls.enabled", true)
		viper.Set("peer.tls.clientAuthRequired", true)
		viper.Set("peer.tls.cert.file", certPath)
		viper.Set("peer.tls.key.file", keyPath)

		coreConfig := deliverservice.GlobalConfig()

		require.True(t, coreConfig.SecOpts.UseTLS)
		require.True(t, coreConfig.SecOpts.RequireClientCert)
		require.Equal(t, keyBytes, coreConfig.SecOpts.Key)
		require.Equal(t, certBytes, coreConfig.SecOpts.Certificate)
	})

	t.Run("no cert", func(t *testing.T) {
		viper.Reset()
		defer viper.Reset()

		viper.Set("peer.tls.enabled", true)
		viper.Set("peer.tls.clientAuthRequired", true)

		require.Panics(t, func() { deliverservice.GlobalConfig() })
	})
}

func TestGlobalConfig(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	viper.Set("peer.tls.enabled", true)
	viper.Set("peer.deliveryclient.reConnectBackoffThreshold", "25s")
	viper.Set("peer.deliveryclient.reconnectTotalTimeThreshold", "20s")
	viper.Set("peer.deliveryclient.connTimeout", "10s")
	viper.Set("peer.keepalive.deliveryClient.interval", "5s")
	viper.Set("peer.keepalive.deliveryClient.timeout", "2s")
	viper.Set("peer.deliveryclient.blockCensorshipTimeoutKey", "40s")
	viper.Set("peer.deliveryclient.minimalReconnectInterval", "110ms")

	coreConfig := deliverservice.GlobalConfig()

	expectedConfig := &deliverservice.DeliverServiceConfig{
		BlockGossipEnabled:          true,
		PeerTLSEnabled:              true,
		ReConnectBackoffThreshold:   25 * time.Second,
		BlockCensorshipTimeoutKey:   40 * time.Second,
		MinimalReconnectInterval:    110 * time.Millisecond,
		ReconnectTotalTimeThreshold: 20 * time.Second,
		ConnectionTimeout:           10 * time.Second,
		KeepaliveOptions: comm.KeepaliveOptions{
			ClientInterval:    time.Second * 5,
			ClientTimeout:     time.Second * 2,
			ServerInterval:    time.Hour * 2,
			ServerTimeout:     time.Second * 20,
			ServerMinInterval: time.Minute,
		},
		SecOpts: comm.SecureOptions{
			UseTLS: true,
		},
	}

	require.Equal(t, expectedConfig, coreConfig)
}

func TestGlobalConfigDefault(t *testing.T) {
	viper.Reset()
	defer viper.Reset()

	coreConfig := deliverservice.GlobalConfig()

	expectedConfig := &deliverservice.DeliverServiceConfig{
		BlockGossipEnabled:          true,
		PeerTLSEnabled:              false,
		ReConnectBackoffThreshold:   deliverservice.DefaultReConnectBackoffThreshold,
		ReconnectTotalTimeThreshold: deliverservice.DefaultReConnectTotalTimeThreshold,
		ConnectionTimeout:           deliverservice.DefaultConnectionTimeout,
		KeepaliveOptions:            comm.DefaultKeepaliveOptions,
		BlockCensorshipTimeoutKey:   deliverservice.DefaultBlockCensorshipTimeoutKey,
		MinimalReconnectInterval:    deliverservice.DefaultMinimalReconnectInterval,
	}

	require.Equal(t, expectedConfig, coreConfig)
}

func TestLoadOverridesMap(t *testing.T) {
	defer viper.Reset()

	t.Run("GreenPath", func(t *testing.T) {
		config := `
                  peer:
                    deliveryclient:
                      addressOverrides:
                        - from: addressFrom1
                          to: addressTo1
                          caCertsFile: testdata/cert.pem
                        - from: addressFrom2
                          to: addressTo2
                          caCertsFile: testdata/cert.pem
                `

		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBuffer([]byte(config)))
		require.NoError(t, err)
		res, err := deliverservice.LoadOverridesMap()
		require.NoError(t, err)
		require.Len(t, res, 2)
		ep1, ok := res["addressFrom1"]
		require.True(t, ok)
		require.Equal(t, "addressTo1", ep1.Address)
		ep2, ok := res["addressFrom2"]
		require.True(t, ok)
		require.Equal(t, "addressTo2", ep2.Address)
	})

	t.Run("MissingCAFiles", func(t *testing.T) {
		config := `
                  peer:
                    deliveryclient:
                      addressOverrides:
                        - from: addressFrom1
                          to: addressTo1
                          caCertsFile: missing/cert.pem
                        - from: addressFrom2
                          to: addressTo2
                          caCertsFile: testdata/cert.pem
                `

		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBuffer([]byte(config)))
		require.NoError(t, err)
		res, err := deliverservice.LoadOverridesMap()
		require.NoError(t, err)
		require.Len(t, res, 1)
	})

	t.Run("EmptyCAFiles", func(t *testing.T) {
		config := `
                  peer:
                    deliveryclient:
                      addressOverrides:
                        - from: addressFrom1
                          to: addressTo1
                        - from: addressFrom2
                          to: addressTo2
                `

		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBuffer([]byte(config)))
		require.NoError(t, err)
		res, err := deliverservice.LoadOverridesMap()
		require.NoError(t, err)
		require.Len(t, res, 2)
	})

	t.Run("BadYaml", func(t *testing.T) {
		config := `
                  peer:
                    deliveryclient:
                      addressOverrides: foo
                `

		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBuffer([]byte(config)))
		require.NoError(t, err)
		_, err = deliverservice.LoadOverridesMap()
		require.Error(t, err)
		require.Contains(t, err.Error(), "could not unmarshal peer.deliveryclient.addressOverrides:")
	})

	t.Run("EmptyYaml", func(t *testing.T) {
		config := `
                  peer:
                    deliveryclient:
                `

		viper.Reset()
		viper.SetConfigType("yaml")
		err := viper.ReadConfig(bytes.NewBuffer([]byte(config)))
		require.NoError(t, err)
		res, err := deliverservice.LoadOverridesMap()
		require.NoError(t, err)
		require.Nil(t, res)
	})
}

func TestGlobalConfigCheckDefaultIsSet(t *testing.T) {
	defer viper.Reset()
	cwd, err := os.Getwd()
	require.NoError(t, err, "failed to get current working directory")
	viper.SetConfigFile(filepath.Join(cwd, "testdata", "core.yaml"))

	err = viper.ReadInConfig()
	require.NoError(t, err)

	require.Equal(t, false, viper.IsSet("peer.deliveryclient.blockCensorshipTimeoutKey"))
	require.Equal(t, false, viper.IsSet("peer.deliveryclient.minimalReconnectInterval"))
	require.Equal(t, true, viper.IsSet("peer.deliveryclient.blockGossipEnabled"))

	coreConfig := deliverservice.GlobalConfig()
	require.NoError(t, err)

	expectedConfig := &deliverservice.DeliverServiceConfig{
		BlockGossipEnabled:          true,
		PeerTLSEnabled:              false,
		ReConnectBackoffThreshold:   deliverservice.DefaultReConnectBackoffThreshold,
		ReconnectTotalTimeThreshold: deliverservice.DefaultReConnectTotalTimeThreshold,
		ConnectionTimeout:           deliverservice.DefaultConnectionTimeout,
		KeepaliveOptions:            comm.DefaultKeepaliveOptions,
		BlockCensorshipTimeoutKey:   deliverservice.DefaultBlockCensorshipTimeoutKey,
		MinimalReconnectInterval:    deliverservice.DefaultMinimalReconnectInterval,
	}

	require.Equal(t, coreConfig, expectedConfig)
}
