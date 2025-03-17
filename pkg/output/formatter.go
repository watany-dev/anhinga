package output

import (
	"encoding/csv"
	"fmt"
	"github.com/anhinga/anhinga/pkg/aws"
	"github.com/olekukonko/tablewriter"
	"io"
	"os"
	"strconv"
)

// FormatType represents the output format type
type FormatType string

const (
	// TableFormat represents tabular output format
	TableFormat FormatType = "table"
	
	// CSVFormat represents CSV output format
	CSVFormat FormatType = "csv"
)

// FormatEBSOutput formats and outputs EBS volume information
func FormatEBSOutput(volumes []aws.EBSInfo, format FormatType) error {
	return FormatEBSOutputTo(volumes, format, os.Stdout)
}

// FormatEBSOutputTo formats and outputs EBS volume information to a specified writer
func FormatEBSOutputTo(volumes []aws.EBSInfo, format FormatType, writer io.Writer) error {
	switch format {
	case TableFormat:
		return formatAsTable(volumes, writer)
	case CSVFormat:
		return formatAsCSV(volumes, writer)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// formatAsTable outputs EBS volume information as a table
func formatAsTable(volumes []aws.EBSInfo, writer io.Writer) error {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Volume ID", "Type", "Size (GB)", "State", "Monthly Cost ($)"})

	var totalCost float64
	for _, v := range volumes {
		totalCost += v.Cost
		
		table.Append([]string{
			v.VolumeID,
			v.VolumeType,
			fmt.Sprintf("%d", v.Size),
			v.State,
			fmt.Sprintf("%.2f", v.Cost),
		})
	}

	// Add total cost as the last row
	table.SetFooter([]string{"", "", "", "Total", fmt.Sprintf("%.2f", totalCost)})
	table.SetBorder(true)
	table.SetCaption(true, fmt.Sprintf("Total EBS Monthly Cost: $%.2f", totalCost))
	
	table.Render()
	return nil
}

// formatAsCSV outputs EBS volume information as CSV
func formatAsCSV(volumes []aws.EBSInfo, writer io.Writer) error {
	csvWriter := csv.NewWriter(writer)
	
	// Write header
	if err := csvWriter.Write([]string{"Volume ID", "Type", "Size (GB)", "State", "Monthly Cost ($)"}); err != nil {
		return err
	}

	// Write volume data
	var totalCost float64
	for _, v := range volumes {
		totalCost += v.Cost
		
		if err := csvWriter.Write([]string{
			v.VolumeID,
			v.VolumeType,
			strconv.Itoa(int(v.Size)),
			v.State,
			fmt.Sprintf("%.2f", v.Cost),
		}); err != nil {
			return err
		}
	}

	// Write total as the last row
	if err := csvWriter.Write([]string{"Total", "", "", "", fmt.Sprintf("%.2f", totalCost)}); err != nil {
		return err
	}

	csvWriter.Flush()
	return csvWriter.Error()
}