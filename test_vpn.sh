#!/bin/bash

if ! /usr/bin/curl http://file.rdu.redhat.com/ &> /dev/null; then
    printf "ERROR: Must be on the VPN\n"
fi

curl -X POST -H "Content-Type: application/json" -d '{"Name": "test000", "ImageBuildHash": "b990a2d2-abf3-44f0-be49-731d43cfab92", "ParentHash": "", "BuildDate": "20210608", "BuildNumber": 1, "ImageBuildTarURL": "http://file.rdu.redhat.com/~admiller/rhel_edge_ostree_tars/0.0.0-b990a2d2-abf3-44f0-be49-731d43cfab92-commit.tar", "OSTreeCommit": "ae65525d9b7d8d343d39aad5458b52951e1def73c05c0cb5a625f3b641efe98e", "Arch": "x86_64" }' localhost:3000/api/edge/v1/commits/

curl -X POST -H "Content-Type: application/json" -d '{"Name": "test001", "ImageBuildHash": "802f86fe-812f-4dc3-80d6-17cdd8ffd654", "ParentHash": "", "BuildDate": "20210609", "BuildNumber": 1, "ImageBuildTarURL": "http://file.rdu.redhat.com/~admiller/rhel_edge_ostree_tars/0.0.1-802f86fe-812f-4dc3-80d6-17cdd8ffd654-commit.tar", "OSTreeCommit": "c510383b369d0ecfd39022a303a32ef0516b5971b9a9e428fdee4281d3c122e7", "Arch": "x86_64" }' localhost:3000/api/edge/v1/commits/

curl -X POST -H "Content-Type: application/json" -d '{"UpdateCommitID": 1, "InventoryHosts": "foobar"}' localhost:3000/api/edge/v1/commits/updates/
