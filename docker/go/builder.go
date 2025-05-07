package main 

import (
	"fmt"
	 "nextdeploy/cmd"
	 "os"
)

func main(){
	switch os.Args[1]{
	case "---doocker":
		if err := cmd.BuildDocker(); err != nil {
			fmt.Println("Docker build failed : %v\n", err)
			os.Exit(1)
		}
	default:
		fmt.Println("Available flags:--doocker")
	}
}
