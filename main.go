/*
Copyright © 2025 NAME HERE caynaashow@gmail.com
*/
package main

import (
	"nextdeploy/cmd"
	"nextdeploy/internal/nextcore"
)

func main() {
	err := nextcore.SendFakeData()
	if err != nil {
		panic(err)
	}
	cmd.Execute()
}
