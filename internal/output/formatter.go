package output

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"text/tabwriter"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/watany-dev/anhinga/internal/aws"
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

// createdAtLayout is the date format used for the "Created At" column.
const createdAtLayout = "2006-01-02"

// headerFor returns the column headers for the requested rendering.
func headerFor(showOwner bool) []string {
	if showOwner {
		return []string{"Volume ID", "Type", "Size (GB)", "State", "Created By", "Created At", "Monthly Cost ($)"}
	}
	return []string{"Volume ID", "Type", "Size (GB)", "State", "Monthly Cost ($)"}
}

// rowFor returns a single volume rendered as columns.
func rowFor(v aws.EBSInfo, showOwner bool) []string {
	columnCount := 5
	if showOwner {
		columnCount = 7
	}
	row := make([]string, 0, columnCount)
	row = append(row,
		v.VolumeID,
		v.VolumeType,
		strconv.Itoa(int(v.Size)),
		v.State,
	)
	if showOwner {
		row = append(row, v.CreatedBy, formatCreatedAt(v.CreatedAt))
	}
	return append(row, strconv.FormatFloat(v.Cost, 'f', 2, 64))
}

// formatCreatedAt renders a volume creation timestamp, tolerating a nil value.
func formatCreatedAt(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(createdAtLayout)
}

// escapeUnsafeTerminalCharacters prevents data returned by AWS from injecting
// terminal control sequences into the human-readable table. CSV and JSON keep
// their original values so they remain suitable for machine processing.
func escapeUnsafeTerminalCharacters(value string) string {
	if !containsUnsafeTerminalRune(value) {
		return value
	}

	const hex = "0123456789ABCDEF"
	var escaped strings.Builder
	escaped.Grow(len(value))
	for _, r := range value {
		if !isUnsafeTerminalRune(r) {
			escaped.WriteRune(r)
			continue
		}

		shift := 12
		if r > 0xFFFF {
			escaped.WriteString(`\U`)
			shift = 28
		} else {
			escaped.WriteString(`\u`)
		}
		for ; shift >= 0; shift -= 4 {
			escaped.WriteByte(hex[(r>>shift)&0xF])
		}
	}
	return escaped.String()
}

func containsUnsafeTerminalRune(value string) bool {
	for i := 0; i < len(value); i++ {
		if value[i] < ' ' || value[i] == 0x7F {
			return true
		}
		if value[i] >= utf8.RuneSelf {
			return strings.IndexFunc(value, isUnsafeTerminalRune) != -1
		}
	}
	return false
}

func isUnsafeTerminalRune(r rune) bool {
	return unicode.IsControl(r) || unicode.In(r, unicode.Cf)
}

func escapeTableRow(row []string) []string {
	for i := range row {
		row[i] = escapeUnsafeTerminalCharacters(row[i])
	}
	return row
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
func FormatEBSOutputTo(volumes []aws.EBSInfo, format FormatType, writer io.Writer, showOwner bool) error {
	totalCost := calculateTotalCost(volumes)

	switch format {
	case TableFormat:
		return formatAsTable(volumes, totalCost, writer, showOwner)
	case CSVFormat:
		return formatAsCSV(volumes, totalCost, writer, showOwner)
	case JSONFormat:
		return formatAsJSON(volumes, totalCost, writer)
	default:
		return fmt.Errorf("unsupported format: %s", format)
	}
}

// formatAsTable outputs EBS volume information as a table
func formatAsTable(volumes []aws.EBSInfo, totalCost float64, writer io.Writer, showOwner bool) error {
	header := headerFor(showOwner)
	for i := range header {
		header[i] = strings.ToUpper(header[i])
	}

	table := tabwriter.NewWriter(writer, 0, 4, 2, ' ', 0)
	writeRow := func(row []string) {
		_, _ = fmt.Fprintln(table, strings.Join(row, "\t"))
	}
	writeRow(header)

	for _, volume := range volumes {
		writeRow(escapeTableRow(rowFor(volume, showOwner)))
	}

	total := make([]string, len(header))
	total[len(total)-2] = "TOTAL"
	total[len(total)-1] = strconv.FormatFloat(totalCost, 'f', 2, 64)
	writeRow(total)
	_, _ = fmt.Fprintf(table, "\nTotal EBS Monthly Cost: $%.2f\n", totalCost)
	return table.Flush()
}

// formatAsCSV outputs EBS volume information as CSV
func formatAsCSV(volumes []aws.EBSInfo, totalCost float64, writer io.Writer, showOwner bool) error {
	csvWriter := csv.NewWriter(writer)

	// writeRow wraps csv.Write with common error handling
	writeRow := func(record []string) error {
		if err := csvWriter.Write(record); err != nil {
			return err
		}
		return nil
	}

	// Write header
	header := headerFor(showOwner)
	if err := writeRow(header); err != nil {
		return err
	}

	// Write volume data
	for _, v := range volumes {
		if err := writeRow(rowFor(v, showOwner)); err != nil {
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
