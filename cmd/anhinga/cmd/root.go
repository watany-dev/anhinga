package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/watany-dev/anhinga/internal/aws"
	"github.com/watany-dev/anhinga/internal/output"
)

type volumeLoader func(aws.Options) ([]aws.EBSInfo, error)

type outputFormatter func([]aws.EBSInfo, output.FormatType, io.Writer, ...output.Option) error

func newRootCommand(loadVolumes volumeLoader, formatVolumes outputFormatter) *cobra.Command {
	var region string
	var formatType string
	var showOwner bool

	command := &cobra.Command{
		Use:           "anhinga",
		Short:         "A CLI tool to list and calculate cost of EBS volumes",
		SilenceErrors: true,
		SilenceUsage:  true,
		Long: `Anhinga is a CLI tool that lists EBS volumes and calculates their costs.
It can display information in different formats like table, CSV, or JSON.
Use the -r flag to specify the AWS region, or omit it to use your default AWS configuration.`,
		Args: cobra.NoArgs,
		RunE: func(command *cobra.Command, _ []string) error {
			format := output.FormatType(strings.ToLower(formatType))
			if format != output.TableFormat && format != output.CSVFormat && format != output.JSONFormat {
				return errorsInvalidFormat()
			}

			volumes, err := loadVolumes(aws.Options{
				Region:    region,
				ShowOwner: showOwner,
				OnWarning: func(message string) {
					_, _ = fmt.Fprintln(command.ErrOrStderr(), "warning:", message)
				},
			})
			if err != nil {
				return fmt.Errorf("failed to get EBS volumes: %w", err)
			}

			var options []output.Option
			if showOwner {
				options = append(options, output.WithOwner())
			}
			return formatVolumes(volumes, format, command.OutOrStdout(), options...)
		},
	}

	command.Flags().StringVarP(&region, "region", "r", "", "AWS region (optional, uses AWS SDK default configuration if not specified)")
	command.Flags().StringVarP(&formatType, "format", "f", "table", "Output format (table, csv, or json)")
	command.Flags().BoolVar(&showOwner, "show-owner", false,
		"Resolve who created each volume via CloudTrail (one rate limited API call per volume; requires cloudtrail:LookupEvents, only covers the last 90 days)")

	return command
}

func errorsInvalidFormat() error {
	return fmt.Errorf("format must be either 'table', 'csv', or 'json'")
}

// Execute builds and runs a fresh root command.
func Execute() error {
	return newRootCommand(aws.GetEBSVolumesWithOptions, output.FormatEBSOutputTo).Execute()
}
