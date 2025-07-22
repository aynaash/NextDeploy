package cmd

// import (
// 	"encoding/json"
// 	"fmt"
// 	"io"
// 	"net/http"
// 	"os"
//
// 	"bytes"
// 	"nextdeploy/shared"
// 	"time"
//
// 	"github.com/spf13/cobra"
// )
//
// var registerCmd = &cobra.Command{
// 	Use:   "register",
// 	Short: "Register a new agent with the NextDeploy dashboard",
// 	Run: func(cmd *cobra.Command, args []string) {
// 		dashboardURL, _ := cmd.Flags().GetString("dashboard")
// 		agentName, _ := cmd.Flags().GetString("name")
// 		vpsIP, _ := cmd.Flags().GetString("vps-ip")
//
// 		// Generate or load existing key pair
// 		//TODO: initialize key manager
// 		privateKey, err := shared.LoadOrGenerateKey("agent.key")
// 		if err != nil {
// 			fmt.Printf("Error with keys: %v\n", err)
// 			os.Exit(1)
// 		}
//
// 		// Prepare registration payload
// 		registration := shared.AgentRegistration{
// 			Name:       agentName,
// 			PublicKey:  crypto.PublicKeyToPEM(&privateKey.PublicKey),
// 			VPSIP:      vpsIP,
// 			Registered: time.Now(),
// 		}
//
// 		// Sign the registration
// 		signedReg, err := shared.SignRegistration(registration, privateKey)
// 		if err != nil {
// 			fmt.Printf("Error signing registration: %v\n", err)
// 			os.Exit(1)
// 		}
//
// 		// Send to dashboard
// 		resp, err := sendRegistration(dashboardURL, signedReg)
// 		if err != nil {
// 			fmt.Printf("Registration failed: %v\n", err)
// 			os.Exit(1)
// 		}
//
// 		fmt.Printf("Successfully registered agent %s with ID: %s\n", agentName, resp.AgentID)
// 	},
// }
//
// func sendRegistration(url string, reg shared.SignedAgentRegistration) (*shared.RegistrationResponse, error) {
// 	jsonData, err := json.Marshal(reg)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	resp, err := http.Post(url+"/api/agents/register", "application/json", bytes.NewBuffer(jsonData))
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer resp.Body.Close()
//
// 	body, err := io.ReadAll(resp.Body)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	var response shared.RegistrationResponse
// 	if err := json.Unmarshal(body, &response); err != nil {
// 		return nil, err
// 	}
//
// 	return &response, nil
// }
//
// func init() {
// 	rootCmd.AddCommand(registerCmd)
// 	registerCmd.Flags().String("dashboard", "", "Dashboard URL (e.g., https://dashboard.nextdeploy.com)")
// 	registerCmd.Flags().String("name", "", "Agent name")
// 	registerCmd.Flags().String("vps-ip", "", "VPS public IP address")
// }
