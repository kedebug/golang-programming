#! /bin/bash

echo "starting up"
cd ./lsp
go install
cd ../echoserver
go build
cd ../echoclient
go build
echo "finished"
