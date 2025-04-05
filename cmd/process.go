package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"nextdeploy/utils"

	"github.com/spf13/cobra"
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Process a JSON file",
	Run: func(cmd *cobra.Command, args []string) {
		// Get the absolute path to data.json
		jsonPath := filepath.Join("cmd", "data.json")

		// Open the JSON file
		file, err := os.Open(jsonPath)
		if err != nil {
			fmt.Printf("Error opening file (%s): %v\n", jsonPath, err)
			return
		}
		defer file.Close()

		// Decode JSON data
		var data utils.Data
		if err := json.NewDecoder(file).Decode(&data); err != nil {
			fmt.Printf("Error decoding JSON: %v\n", err)
			return
		}

		// Process data
		result := utils.ProcessData(data, "")

		// Pretty-print the processed JSON output
		formattedJSON, err := json.MarshalIndent(result, "", "  ")
		if err != nil {
			fmt.Printf("Error formatting JSON: %v\n", err)
			return
		}

		fmt.Println("Processed Data:")
		fmt.Println(string(formattedJSON))
	},
}

func init() {
	rootCmd.AddCommand(processCmd)
}
