# Copyright (c) 2023, Oracle and/or its affiliates. All rights reserved.

export GO111MODULE=on

build:
#	go get -u golang.org/x/lint/golint
	golint -set_exit_status ./...
	go test -v ./...

coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
