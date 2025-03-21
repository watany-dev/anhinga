package output

import (
	"bytes"
	"github.com/anhinga/anhinga/internal/aws"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFormatEBSOutput(t *testing.T) {
	// This test only verifies that the function doesn't panic
	// since we can't easily capture os.Stdout output
	err := FormatEBSOutput(getTestVolumes(), TableFormat)
	assert.NoError(t, err)
	
	err = FormatEBSOutput(getTestVolumes(), FormatType("invalid"))
	assert.Error(t, err)
}

func TestFormatEBSOutputTo(t *testing.T) {
	tests := []struct {
		name          string
		volumes       []aws.EBSInfo
		format        FormatType
		expectedError bool
	}{
		{
			name:          "Table Format Valid",
			volumes:       getTestVolumes(),
			format:        TableFormat,
			expectedError: false,
		},
		{
			name:          "CSV Format Valid",
			volumes:       getTestVolumes(),
			format:        CSVFormat,
			expectedError: false,
		},
		{
			name:          "JSON Format Valid",
			volumes:       getTestVolumes(),
			format:        JSONFormat,
			expectedError: false,
		},
		{
			name:          "Unsupported Format",
			volumes:       getTestVolumes(),
			format:        FormatType("invalid"),
			expectedError: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			buffer := &bytes.Buffer{}
			err := FormatEBSOutputTo(tc.volumes, tc.format, buffer)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotEmpty(t, buffer.String())
			}
		})
	}
}

func TestFormatAsTable(t *testing.T) {
	volumes := getTestVolumes()
	buffer := &bytes.Buffer{}

	err := formatAsTable(volumes, buffer)
	assert.NoError(t, err)
	
	output := buffer.String()
	
	// Check that the output contains expected elements
	assert.Contains(t, output, "VOLUME ID")
	assert.Contains(t, output, "TYPE")
	assert.Contains(t, output, "SIZE (GB)")
	assert.Contains(t, output, "STATE")
	assert.Contains(t, output, "MONTHLY COST ($)")
	
	// Check for volume data
	assert.Contains(t, output, "vol-123")
	assert.Contains(t, output, "vol-456")
	
	// Verify total is included
	assert.Contains(t, output, "TOTAL")
	assert.Contains(t, output, "17.00") // Combined cost of test volumes
}

func TestFormatAsCSV(t *testing.T) {
	volumes := getTestVolumes()
	buffer := &bytes.Buffer{}

	err := formatAsCSV(volumes, buffer)
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
	
	// Test with writer that always fails
	alwaysFailsWriter := &badWriter{}
	err := formatAsCSV(volumes, alwaysFailsWriter)
	assert.Error(t, err)
}

func TestFormatAsJSON(t *testing.T) {
	volumes := getTestVolumes()
	buffer := &bytes.Buffer{}

	err := formatAsJSON(volumes, buffer)
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