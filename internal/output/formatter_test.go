package output

import (
	"bytes"
	"github.com/stretchr/testify/assert"
	"github.com/watany-dev/anhinga/internal/aws"
	"testing"
)

func TestCalculateTotalCost(t *testing.T) {
	volumes := getTestVolumes()
	totalCost := calculateTotalCost(volumes)
	assert.Equal(t, 17.0, totalCost, "Total cost calculation should match expected value")
}

func TestFormatEBSOutputTo(t *testing.T) {
	for _, format := range []FormatType{TableFormat, CSVFormat, JSONFormat} {
		t.Run(string(format), func(t *testing.T) {
			buffer := &bytes.Buffer{}
			assert.NoError(t, FormatEBSOutputTo(getTestVolumes(), format, buffer, false))
			assert.NotEmpty(t, buffer.String())
		})
	}
	assert.Error(t, FormatEBSOutputTo(getTestVolumes(), FormatType("invalid"), &bytes.Buffer{}, false))
}

func TestFormatAsTablePreservesLayout(t *testing.T) {
	buffer := &bytes.Buffer{}
	err := formatAsTable(getTestVolumes(), 17, buffer, false)
	assert.NoError(t, err)

	expected := `+-----------+------+-----------+-----------+------------------+
| VOLUME ID | TYPE | SIZE (GB) |   STATE   | MONTHLY COST ($) |
+-----------+------+-----------+-----------+------------------+
| vol-123   | gp2  |       100 | available |            10.00 |
| vol-456   | io1  |        70 | available |             7.00 |
+-----------+------+-----------+-----------+------------------+
|                                  TOTAL   |      17.00       |
+-----------+------+-----------+-----------+------------------+
Total EBS Monthly Cost: $17.00
`
	assert.Equal(t, expected, buffer.String())
}

func TestFormatAsTableReportsWriterErrors(t *testing.T) {
	err := formatAsTable(getTestVolumes(), 17, &badWriter{}, false)
	assert.Error(t, err)
}

func TestFormatAsCSV(t *testing.T) {
	volumes := getTestVolumes()
	buffer := &bytes.Buffer{}
	totalCost := calculateTotalCost(volumes)

	err := formatAsCSV(volumes, totalCost, buffer, false)
	assert.NoError(t, err)

	output := buffer.String()

	// Check expected content
	assert.Contains(t, output, "Volume ID,Type,Size (GB),State,Monthly Cost ($)")
	assert.Contains(t, output, "vol-123,gp2,100,available,10.00")
	assert.Contains(t, output, "vol-456,io1,70,available,7.00")
	assert.Contains(t, output, "Total,,,,17.00")
}

func TestFormatAsCSVErrorHandling(t *testing.T) {
	volumes := getTestVolumes()
	totalCost := calculateTotalCost(volumes)

	// Test with writer that always fails
	alwaysFailsWriter := &badWriter{}
	err := formatAsCSV(volumes, totalCost, alwaysFailsWriter, false)
	assert.Error(t, err)
}

func TestFormatAsJSON(t *testing.T) {
	volumes := getTestVolumes()
	buffer := &bytes.Buffer{}
	totalCost := calculateTotalCost(volumes)

	err := formatAsJSON(volumes, totalCost, buffer)
	assert.NoError(t, err)

	output := buffer.String()

	// Check expected content
	assert.Contains(t, output, `"volumes":`)
	assert.Contains(t, output, `"volumeId": "vol-123"`)
	assert.Contains(t, output, `"volumeId": "vol-456"`)
	assert.Contains(t, output, `"totalCost": 17`)
}

// Helper to create test volume data
func getTestVolumes() []aws.EBSInfo {
	return []aws.EBSInfo{
		{
			VolumeID:   "vol-123",
			VolumeType: "gp2",
			Size:       100,
			State:      "available",
			Cost:       10.0,
		},
		{
			VolumeID:   "vol-456",
			VolumeType: "io1",
			Size:       70,
			State:      "available",
			Cost:       7.0,
		},
	}
}
