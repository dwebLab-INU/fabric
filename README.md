# WEAVE: Enhancing Decentralization Property of Hyperledger Fabric Blockchain

To make Fabric blockchain more reliable and trustworthy by connecting it to a programmable distributed offchain storage and another blockchain network.



## Prerequisites(Only in linux)

#### Docker 

Install the latest version of Docker if it is not already installed.

  ```sudo apt-get -y install docker-compose``` 

Add your user to the Docker group.

  ```sudo usermod -a -G docker <username>``` 

#### Go

Install at https://go.dev/doc/install

#### JQ

Install the latest version of jq if it is not already installed (only required for the tutorials related to channel configuration transactions).

```sudo apt-get install jq```

#### Mongodb

```
sudo apt-get install gnupg curl
curl -fsSL https://www.mongodb.org/static/pgp/server-7.0.asc | \
   sudo gpg -o /usr/share/keyrings/mongodb-server-7.0.gpg \
   --dearmor
echo "deb [ arch=amd64,arm64 signed-by=/usr/share/keyrings/mongodb-server-7.0.gpg ] https://repo.mongodb.org/apt/ubuntu jammy/mongodb-org/7.0 multiverse" | sudo tee /etc/apt/sources.list.d/mongodb-org-7.0.list
sudo apt-get update
sudo apt-get install -y mongodb-org

```


### Download Fabric Docker images, and fabric binaries

To get the install script:

```curl -sSLO https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh && chmod +x install-fabric.sh```

We only need docker images and binaries, not samples.
```
./install-fabric.sh d b
or
./install-fabric.sh docker binary
```

### Prepare docker images

You can pull kafka, snode, wnode docker images

```
docker pull ghcr.io/dweblab-inu/kafka:v1
docker pull ghcr.io/dweblab-inu/snode:v1
docker pull ghcr.io/dweblab-inu/wnode:v1
```

### Start a kafka broker:

Before start broker, you have to write IPAddress in .env file.

.env
```
KAFKA_BROKER1={BrokerIP}:9091
KAFKA_BROKER2={BrokerIP}:9092
KAFKA_BROKER3={BrokerIP}:9093
```

```docker-compose up -d```

The command to stop broker

```docker-compose down```

## How to start

Deploy kafka-realtime processor

 e.g. docker run -it --name kafka ghcr.io/dweb-lab/kafka:v1 ./src/kafka.sh 192.168.0.12
```
docker run -it --name kafka ghcr.io/jhikyuinn/kafka:1.0 ./src/kafka.sh {BrokerIP}
```

Deploy watchdog snode

e.g. docker run -it --network host --name snode ghcr.io/dweb-lab/snode:v1 ./src/snode.sh 192.168.0.12 192.168.0.12
```
docker run -it --network host --name snode ghcr.io/dweb-lab/snode:1.0 ./src/snode.sh {snodeIP} {processorIP}
```

Deploy watchdog wnode

e.g. docker run -it  --network host --name wnode ghcr.io/dweb-lab/snode:1.0 ./src/wnode.sh 192.168.0.12 192.168.0.12 192.168.0.12 QmVjm73FcrFU7TQ6D5sae7UCoKuoaftLjLdpRu3FscDz4Z
```
docker run -it --network host --name wnode ghcr.io/dweb-lab/wnode:1.0 ./src/wnode.sh {snodeIP} {BrokerIP} {wnodeIP} {snodePeerID}
```

Deploy fabric 

```
cd fabric
make peer-docker
```

You can test the fabric network.

```
cd fabric-samples/test-network
./start.sh
./invoke.sh 
```

The command to stop fabric network

```./network down```
