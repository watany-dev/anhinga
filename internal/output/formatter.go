package output

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"github.com/mattn/go-runewidth"
	"github.com/watany-dev/anhinga/internal/aws"
	"io"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"
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

	widths := make([]int, len(header))
	for i, cell := range header {
		widths[i] = displayWidth(cell)
	}

	rows := make([][]string, len(volumes))
	for i, v := range volumes {
		row := escapeTableRow(rowFor(v, showOwner))
		rows[i] = row
		for column, cell := range row {
			widths[column] = max(widths[column], displayWidth(cell))
		}
	}

	buffer := bufio.NewWriterSize(writer, 64*1024)
	writeBorder(buffer, widths)
	writeTableRow(buffer, header, widths, tableHeader)
	writeBorder(buffer, widths)
	for _, row := range rows {
		writeTableRow(buffer, row, widths, tableBody)
	}
	writeBorder(buffer, widths)
	writeTableFooter(buffer, widths, strconv.FormatFloat(totalCost, 'f', 2, 64))
	writeBorder(buffer, widths)
	_, _ = fmt.Fprintf(buffer, "Total EBS Monthly Cost: $%.2f\n", totalCost)

	return buffer.Flush()
}

type tableRowKind uint8

const (
	tableHeader tableRowKind = iota
	tableBody
)

func writeBorder(writer *bufio.Writer, widths []int) {
	_ = writer.WriteByte('+')
	for _, width := range widths {
		_, _ = writer.WriteString(strings.Repeat("-", width+2))
		_ = writer.WriteByte('+')
	}
	_ = writer.WriteByte('\n')
}

func writeTableRow(writer *bufio.Writer, row []string, widths []int, kind tableRowKind) {
	_ = writer.WriteByte('|')
	for i, cell := range row {
		_ = writer.WriteByte(' ')
		alignment := alignLeft
		if kind == tableHeader {
			alignment = alignCenter
		} else if i == 2 || i == len(row)-1 {
			alignment = alignRight
		}
		writePadded(writer, cell, widths[i], alignment)
		_, _ = writer.WriteString(" |")
	}
	_ = writer.WriteByte('\n')
}

func writeTableFooter(writer *bufio.Writer, widths []int, total string) {
	prefixWidth := 0
	for i := 0; i < len(widths)-2; i++ {
		prefixWidth += widths[i]
	}
	prefixWidth += 3 * (len(widths) - 2)

	_, _ = writer.WriteString("| ")
	_, _ = writer.WriteString(strings.Repeat(" ", prefixWidth))
	writePadded(writer, "TOTAL", widths[len(widths)-2], alignCenter)
	_, _ = writer.WriteString(" | ")
	writePadded(writer, total, widths[len(widths)-1], alignCenter)
	_, _ = writer.WriteString(" |\n")
}

type tableAlignment uint8

const (
	alignLeft tableAlignment = iota
	alignCenter
	alignRight
)

func writePadded(writer *bufio.Writer, value string, width int, alignment tableAlignment) {
	padding := width - displayWidth(value)
	left := 0
	right := padding
	switch alignment {
	case alignCenter:
		left = padding / 2
		right = padding - left
	case alignRight:
		left = padding
		right = 0
	}

	writeSpaces(writer, left)
	_, _ = writer.WriteString(value)
	writeSpaces(writer, right)
}

func displayWidth(value string) int {
	for i := 0; i < len(value); i++ {
		if value[i] >= utf8.RuneSelf {
			return runewidth.StringWidth(value)
		}
	}
	return len(value)
}

func writeSpaces(writer *bufio.Writer, count int) {
	const spaces = "                                                                "
	for count > len(spaces) {
		_, _ = writer.WriteString(spaces)
		count -= len(spaces)
	}
	_, _ = writer.WriteString(spaces[:count])
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
