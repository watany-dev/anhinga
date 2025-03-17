package cmd

import (
	"fmt"
	"strings"

	"github.com/anhinga/anhinga/pkg/aws"
	"github.com/anhinga/anhinga/pkg/output"
	"github.com/spf13/cobra"
)

var (
	region     string
	formatType string
)

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "anhinga",
	Short: "A CLI tool to list and calculate cost of EBS volumes",
	Long: `Anhinga is a CLI tool that lists EBS volumes and calculates their costs.
It can display information in different formats like table, CSV, or JSON.
Use the -r flag to specify the AWS region, or omit it to use your default AWS configuration.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Validate format
		format := output.FormatType(strings.ToLower(formatType))
		if format != output.TableFormat && format != output.CSVFormat && format != output.JSONFormat {
			return fmt.Errorf("format must be either 'table', 'csv', or 'json'")
		}

		// Get EBS volumes
		volumes, err := aws.GetEBSVolumes(region)
		if err != nil {
			return fmt.Errorf("failed to get EBS volumes: %v", err)
		}

		// Format and display output
		return output.FormatEBSOutput(volumes, format)
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	// Define flags
	rootCmd.Flags().StringVarP(&region, "region", "r", "", "AWS region (optional, uses AWS SDK default configuration if not specified)")
	rootCmd.Flags().StringVarP(&formatType, "format", "f", "table", "Output format (table, csv, or json)")

	// Region flag is now optional
}