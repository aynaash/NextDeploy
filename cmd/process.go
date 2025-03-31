package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"NextOperations/utils"

	"github.com/spf13/cobra"
)

var secret string

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process a JSON file",
	Run: func(cmd *cobra.Command, args []string) {
		file, err := os.Open("data.json")
		if err != nil {
			fmt.Println("Error opening file:", err)
			return
		}
		defer file.Close()

		var data utils.Data
		if err := json.NewDecoder(file).Decode(&data); err != nil {
			fmt.Println("Error decoding JSON:", err)
			return
		}

		// Fetch secrets if not provided via flag
		if secret == "" {
			secret = utils.GetSecret("APPNAME")
		}

		result := utils.ProcessData(data, secret)
		fmt.Printf("Processed data: %+v\n", result)
	},
}

func init() {
	processCmd.Flags().StringVarP(&secret, "secret", "s", "", "Secret for processing")
	rootCmd.AddCommand(processCmd)
}
