package main 

import (
	"encoding/json"
	"fmt"
	"os"
)

// define the expected json structure
 type Config struct {
	 AppName string `json:"name"`
	 Value  int     `json:"value"`
	 // ipaddress, sshkey, container container 31/03/2025
	 
 }

 // function to process the Config
 
 func processConfig(d, Config) Config {
	 d.value *= 2
	 return d
 }

 // function check if docker file exits
 
 func dockerFileExists() bool {
	 _, err := os.Stat("DockerFile")
	 return err == nill
 }

 // function to get the laest git commit hash
 
 func getGitCommitHash()(string, error){
	 cmd := exec.Command("git"m "rev-parse", "HEAD")
	 var out bytes.Buffer
	 cmd.Stdout = &out
	 cmd.stderr = &out

	 if err := cmd.Run(); err != nil {
		 return "", fmt.Error("failed to get fit commit hash please label manully or retyr");
		 // TODO: make user pass a hash or name for the image

	 }

 }

func main(){
	print("NextOperations first line of code");
}
