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
	"time"
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

// Option customizes how the output is rendered.
type Option func(*renderOptions)

type renderOptions struct {
	showOwner bool
}

// WithOwner adds the "Created By" and "Created At" columns, populated from the
// CloudTrail lookup.
func WithOwner() Option {
	return func(o *renderOptions) {
		o.showOwner = true
	}
}

func newRenderOptions(opts []Option) renderOptions {
	var o renderOptions
	for _, apply := range opts {
		apply(&o)
	}
	return o
}

// createdAtLayout is the date format used for the "Created At" column.
const createdAtLayout = "2006-01-02"

// FormatEBSOutput formats and outputs EBS volume information
func FormatEBSOutput(volumes []aws.EBSInfo, format FormatType, opts ...Option) error {
	return FormatEBSOutputTo(volumes, format, os.Stdout, opts...)
}

// headerFor returns the column headers for the requested rendering.
func headerFor(o renderOptions) []string {
	if o.showOwner {
		return []string{"Volume ID", "Type", "Size (GB)", "State", "Created By", "Created At", "Monthly Cost ($)"}
	}
	return []string{"Volume ID", "Type", "Size (GB)", "State", "Monthly Cost ($)"}
}

// rowFor returns a single volume rendered as columns.
func rowFor(v aws.EBSInfo, o renderOptions) []string {
	row := []string{
		v.VolumeID,
		v.VolumeType,
		strconv.Itoa(int(v.Size)),
		v.State,
	}
	if o.showOwner {
		row = append(row, v.CreatedBy, formatCreatedAt(v.CreatedAt))
	}
	return append(row, fmt.Sprintf("%.2f", v.Cost))
}

// formatCreatedAt renders a volume creation timestamp, tolerating a nil value.
func formatCreatedAt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(createdAtLayout)
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
func FormatEBSOutputTo(volumes []aws.EBSInfo, format FormatType, writer io.Writer, opts ...Option) error {
	totalCost := calculateTotalCost(volumes)
	renderOpts := newRenderOptions(opts)

	switch format {
	case TableFormat:
		return formatAsTable(volumes, totalCost, writer, renderOpts)
	case CSVFormat:
		return formatAsCSV(volumes, totalCost, writer, renderOpts)
	case JSONFormat:
		return formatAsJSON(volumes, totalCost, writer)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// formatAsTable outputs EBS volume information as a table
func formatAsTable(volumes []aws.EBSInfo, totalCost float64, writer io.Writer, o renderOptions) error {
	table := tablewriter.NewWriter(writer)
	header := headerFor(o)
	table.SetHeader(header)

	for _, v := range volumes {
		table.Append(rowFor(v, o))
	}

	// Add total cost as the last row
	footer := make([]string, len(header))
	footer[len(footer)-2] = "Total"
	footer[len(footer)-1] = fmt.Sprintf("%.2f", totalCost)
	table.SetFooter(footer)
	table.SetBorder(true)
	table.SetCaption(true, fmt.Sprintf("Total EBS Monthly Cost: $%.2f", totalCost))

	table.Render()
	return nil
}

// formatAsCSV outputs EBS volume information as CSV
func formatAsCSV(volumes []aws.EBSInfo, totalCost float64, writer io.Writer, o renderOptions) error {
	csvWriter := csv.NewWriter(writer)

	// writeRow wraps csv.Write with common error handling
	writeRow := func(record []string) error {
		if err := csvWriter.Write(record); err != nil {
			return err
		}
		return nil
	}

	// Write header
	header := headerFor(o)
	if err := writeRow(header); err != nil {
		return err
	}

	// Write volume data
	for _, v := range volumes {
		if err := writeRow(rowFor(v, o)); err != nil {
			return err
		}
	}

	// Write total as the last row
	total := make([]string, len(header))
	total[0] = "Total"
	total[len(total)-1] = fmt.Sprintf("%.2f", totalCost)
	if err := writeRow(total); err != nil {
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
