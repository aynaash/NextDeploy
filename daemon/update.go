package main

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

func UpdateDaemon() {
	owner := "aynaash"
	repo := "NextDeploy"
	latestURL := fmt.Sprintf("https://github.com/%s/%s/releases/latest/download/nextdeployd-%s-%s", owner, repo, runtime.GOOS, runtime.GOARCH)

	fmt.Println("Fetching latest daemon release from:", latestURL)

	cmd := exec.Command("curl", "-L", latestURL, "-o", "/usr/local/bin/nextdeployd")
	cmd.Stdout = os.Stdout

	cmd.Stderr = os.Stderr

	err := cmd.Run()
	if err != nil {
		fmt.Println("Error downloading the latest daemon version:", err)
		return
	}

	exec.Command("chmod", "+x", "/usr/local/bin/nextdeployd").Run()
	fmt.Println("NextDeploy Daemon has been updated to the latest version!")

	restart := exec.Command("sudo", "systemctl", "restart", "nextdeployd")
	restart.Stdout = os.Stdout
	restart.Stderr = os.Stderr
	err = restart.Run()

	if err != nil {
		fmt.Println("Error restarting the NextDeploy Daemon:", err)
		return
	}
	fmt.Println("NextDeploy Daemon has been restarted successfully!")

}
