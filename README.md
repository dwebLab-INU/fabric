# WEAVE: Enhancing Decentralization Property of Hyperledger Fabric Blockchain

To make Fabric blockchain more reliable and trustworthy by connecting it to a programmable distributed offchain storage and another blockchain network.



## Prerequisites(Only in linux)

#### Docker 

Install the latest version of Docker if it is not already installed.

  ```sudo apt-get -y install docker-compose``` 

Add your user to the Docker group.

  ```sudo usermod -a -G docker <username>``` 

#### Go

Install the latest version of Go if it is not already installed (only required if you will be writing Go chaincode or SDK applications).

#### JQ

Install the latest version of jq if it is not already installed (only required for the tutorials related to channel configuration transactions).
```apt-get install JQ```


### Download Fabric Docker images, and fabric binaries

To get the install script:

```curl -sSLO https://raw.githubusercontent.com/hyperledger/fabric/main/scripts/install-fabric.sh && chmod +x install-fabric.sh```

We only need docker images and binaries, not samples
```
./install-fabric.sh d b
or
./install-fabric.sh docker binary
```

### Prepare docker images

```
docker load -i kafka.tar
docker load -i snode.tar
docker load -i wnode.tar
```

### Start a kafka broker:

```docker-compose up -d```

.env
```
KAFKA_BROKER1={YOUR_IP}:9091
KAFKA_BROKER2={YOUR_IP}:9092
KAFKA_BROKER3={YOUR_IP}:9093
```

## How to start


kafka-realtime processor > watchdog snode > watchdog wnode > fabric

```
docker run -it --name kafka kafka:1.0 ./src/kafka.sh {brokerIP}
ex) docker run -it --name kafka kafka:1.0 ./src/kafka.sh 172.17.0.2
```

```
docker run -it --privileged --add-host host.docker.internal:{snodeIP} --name snode s-node:1.0 ./src/snode.sh {processorIP}
ex) docker run -it --privileged --add-host host.docker.internal:172.17.0.3 --name snode s-node:1.0 ./src/snode.sh 172.17.0.2
```

```
docker run -it  --privileged --add-host host.docker.internal:{wnodeIP} --name wnode w-node:1.0 ./src/wnode.sh {snodeIP} {brokerIP} {wnodeIP} {snodePeerID}
ex) docker run -it  --privileged --add-host host.docker.internal:172.17.0.4 --name wnode w-node:1.0 ./src/wnode.sh 172.17.0.3 192.168.0.12 172.17.0.3 QmdB8Uyuvv5TZbs8Rxi8VefgxkrSZQzVgFf1Adq4Ce8NpD
```

```
cd fabric
make orderer-docker
make peer-docker
```

```
cd fabric-samples/test-network
./start.sh
./invokes.sh
```
