package main

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
)

func runCommand() {
	cmd := exec.Command(
		"peer", "chaincode", "invoke",
		"-o", "localhost:7050",
		"--ordererTLSHostnameOverride", "orderer.example.com",
		"--tls",
		"--cafile", os.Getenv("PWD")+"/organizations/ordererOrganizations/example.com/orderers/orderer.example.com/msp/tlscacerts/tlsca.example.com-cert.pem",
		"-C", "mychannel",
		"-n", "basic",
		"--peerAddresses", "localhost:7051",
		"--tlsRootCertFiles", os.Getenv("PWD")+"/organizations/peerOrganizations/org1.example.com/peers/peer0.org1.example.com/tls/ca.crt",
		"--peerAddresses", "localhost:9051",
		"--tlsRootCertFiles", os.Getenv("PWD")+"/organizations/peerOrganizations/org2.example.com/peers/peer0.org2.example.com/tls/ca.crt",
		"-c", `{"function":"TransferAsset","Args":["asset6","Tomas"]}`,
	)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error:", err)
	}
}

func main() {
	var wg sync.WaitGroup
	//change tx per block number
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			runCommand()
		}()
	}

	wg.Wait()
}
