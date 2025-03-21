package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/anhinga/anhinga/internal/aws"
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
	
	// JSONFormat represents JSON output format
	JSONFormat FormatType = "json"
)

// FormatEBSOutput formats and outputs EBS volume information
func FormatEBSOutput(volumes []aws.EBSInfo, format FormatType) error {
	return FormatEBSOutputTo(volumes, format, os.Stdout)
}

// calculateTotalCost calculates the total cost of all volumes
func calculateTotalCost(volumes []aws.EBSInfo) float64 {
	var totalCost float64
	for _, v := range volumes {
		totalCost += v.Cost
	}
	return totalCost
}

// FormatEBSOutputTo formats and outputs EBS volume information to a specified writer
func FormatEBSOutputTo(volumes []aws.EBSInfo, format FormatType, writer io.Writer) error {
	totalCost := calculateTotalCost(volumes)
	
	switch format {
	case TableFormat:
		return formatAsTable(volumes, totalCost, writer)
	case CSVFormat:
		return formatAsCSV(volumes, totalCost, writer)
	case JSONFormat:
		return formatAsJSON(volumes, totalCost, writer)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// formatAsTable outputs EBS volume information as a table
func formatAsTable(volumes []aws.EBSInfo, totalCost float64, writer io.Writer) error {
	table := tablewriter.NewWriter(writer)
	table.SetHeader([]string{"Volume ID", "Type", "Size (GB)", "State", "Monthly Cost ($)"})

	for _, v := range volumes {
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
func formatAsCSV(volumes []aws.EBSInfo, totalCost float64, writer io.Writer) error {
	csvWriter := csv.NewWriter(writer)
	
	// writeRow wraps csv.Write with common error handling
	writeRow := func(record []string) error {
		if err := csvWriter.Write(record); err != nil {
			return err
		}
		return nil
	}
	
	// Write header
	if err := writeRow([]string{"Volume ID", "Type", "Size (GB)", "State", "Monthly Cost ($)"}); err != nil {
		return err
	}

	// Write volume data
	for _, v := range volumes {
		if err := writeRow([]string{
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
	if err := writeRow([]string{"Total", "", "", "", fmt.Sprintf("%.2f", totalCost)}); err != nil {
		return err
	}

	csvWriter.Flush()
	return csvWriter.Error()
}

// formatAsJSON outputs EBS volume information as JSON
func formatAsJSON(volumes []aws.EBSInfo, totalCost float64, writer io.Writer) error {
	type jsonOutput struct {
		Volumes   []aws.EBSInfo `json:"volumes"`
		TotalCost float64       `json:"totalCost"`
	}
	
	output := jsonOutput{
		Volumes:   volumes,
		TotalCost: totalCost,
	}
	
	encoder := json.NewEncoder(writer)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}
