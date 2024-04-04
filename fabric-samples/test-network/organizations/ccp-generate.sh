#!/bin/bash

function one_line_pem {
    echo "`awk 'NF {sub(/\\n/, ""); printf "%s\\\\\\\n",$0;}' $1`"
}

function json_ccp {
    local PP=$(one_line_pem ${13})
    local CP=$(one_line_pem ${14})
    sed -e "s/\${ORG}/$1/" \
        -e "s/\${P0PORT}/$2/" \
        -e "s/\${P1PORT}/$3/" \
        -e "s/\${P2PORT}/$4/" \
        -e "s/\${P3PORT}/$5/" \
        -e "s/\${P4PORT}/$6/" \
        -e "s/\${P5PORT}/$7/" \
        -e "s/\${P6PORT}/$8/" \
        -e "s/\${P7PORT}/$9/" \
        -e "s/\${P8PORT}/${10}/" \
        -e "s/\${P9PORT}/${11}/" \
        -e "s/\${CAPORT}/${12}/" \
        -e "s#\${PEERPEM}#$PP#" \
        -e "s#\${CAPEM}#$CP#" \
        organizations/ccp-template.json
}

function yaml_ccp {
    local PP=$(one_line_pem ${13})
    local CP=$(one_line_pem ${14})
    sed -e "s/\${ORG}/$1/" \
        -e "s/\${P0PORT}/$2/" \
        -e "s/\${P1PORT}/$3/" \
        -e "s/\${P2PORT}/$4/" \
        -e "s/\${P3PORT}/$5/" \
        -e "s/\${P4PORT}/$6/" \
        -e "s/\${P5PORT}/$7/" \
        -e "s/\${P6PORT}/$8/" \
        -e "s/\${P7PORT}/$9/" \
        -e "s/\${P8PORT}/${10}/" \
        -e "s/\${P9PORT}/${11}/" \
        -e "s/\${CAPORT}/${12}/" \
        -e "s#\${PEERPEM}#$PP#" \
        -e "s#\${CAPEM}#$CP#" \
        organizations/ccp-template.yaml | sed -e $'s/\\\\n/\\\n          /g'
}

ORG=1
P0PORT=7051
P1PORT=7151
P2PORT=7251
P3PORT=7351
P4PORT=7451
P5PORT=7551
P6PORT=7651
P7PORT=7751
P8PORT=7851
P9PORT=7951
CAPORT=7054
PEERPEM=organizations/peerOrganizations/org1.example.com/tlsca/tlsca.org1.example.com-cert.pem
CAPEM=organizations/peerOrganizations/org1.example.com/ca/ca.org1.example.com-cert.pem

echo "$(json_ccp $ORG $P0PORT $P1PORT $P2PORT $P3PORT $P4PORT $P5PORT $P6PORT $P7PORT $P8PORT $P9PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/org1.example.com/connection-org1.json
echo "$(yaml_ccp $ORG $P0PORT $P1PORT $P2PORT $P3PORT $P4PORT $P5PORT $P6PORT $P7PORT $P8PORT $P9PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/org1.example.com/connection-org1.yaml

ORG=2
P0PORT=9051
P1PORT=9151
P2PORT=9251
P3PORT=9351
P4PORT=9451
P5PORT=9551
P6PORT=9651
P7PORT=9751
P8PORT=9851
P9PORT=9951
CAPORT=8054
PEERPEM=organizations/peerOrganizations/org2.example.com/tlsca/tlsca.org2.example.com-cert.pem
CAPEM=organizations/peerOrganizations/org2.example.com/ca/ca.org2.example.com-cert.pem

echo "$(json_ccp $ORG $P0PORT $P1PORT $P2PORT $P3PORT $P4PORT $P5PORT $P6PORT $P7PORT $P8PORT $P9PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/org2.example.com/connection-org2.json
echo "$(yaml_ccp $ORG $P0PORT $P1PORT $P2PORT $P3PORT $P4PORT $P5PORT $P6PORT $P7PORT $P8PORT $P9PORT $CAPORT $PEERPEM $CAPEM)" > organizations/peerOrganizations/org2.example.com/connection-org2.yaml