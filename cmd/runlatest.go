
package cmd

import (
	"fmt"

	"github.com/shirou/gopsutil/v4/mem"
	"github.com/spf13/cobra"
)

var memCmd = &cobra.Command{
	Use:   "runlatest",
	Short: "Runs the latest container using blue-green deployment",
	Run: func(cmd *cobra.Command, args []string) {
		v, err := mem.VirtualMemory()
		if err != nil {
			fmt.Println("❌ Failed to fetch memory stats:", err)
			return
		}

		fmt.Printf("🧠 Memory Info:\n")
		fmt.Printf("Total:       %v MB\n", v.Total/1024/1024)
		fmt.Printf("Free:        %v MB\n", v.Free/1024/1024)
		fmt.Printf("Used:        %v MB\n", v.Used/1024/1024)
		fmt.Printf("UsedPercent: %.2f%%\n", v.UsedPercent)
	},
}

func init() {
	rootCmd.AddCommand(memCmd)
}
