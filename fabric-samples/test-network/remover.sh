docker exec peer0.org1.example.com tc qdisc del dev eth0 root

docker exec peer1.org1.example.com tc qdisc del dev eth0 root

docker exec peer2.org1.example.com tc qdisc del dev eth0 root

docker exec peer0.org2.example.com tc qdisc del dev eth0 root

docker exec peer1.org2.example.com tc qdisc del dev eth0 root