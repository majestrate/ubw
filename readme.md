# Unlimited Bot Works

This repo is for archer class session bots.

note: The Archer class really is made up of archers.

## requirements

* go >= 1.16
* protoc
* protoc-gen-go

## building

check source code out:

    $ git clone https://github.com/majestrate/ubw
    $ cd ubw
    
building the bot:

    $ go generate ./...
    $ go build -tags dontbrickme ./cmd/archer

## running

echobot:

    $ ./archer

custom message handler:

    $ ./archer ./example/reply.sh
