package output

import (
	"bytes"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/watany-dev/anhinga/internal/aws"
)

// getOwnerTestVolumes returns volumes carrying creator information.
func getOwnerTestVolumes() []aws.EBSInfo {
	created := time.Date(2026, 3, 1, 12, 0, 0, 0, time.UTC)
	return []aws.EBSInfo{
		{
			VolumeID:   "vol-123",
			VolumeType: "gp2",
			Size:       100,
			State:      "available",
			Cost:       10.0,
			CreatedAt:  &created,
			CreatedBy:  "DeployRole/alice",
		},
		{
			VolumeID:   "vol-456",
			VolumeType: "io1",
			Size:       70,
			State:      "available",
			Cost:       7.0,
			CreatedBy:  "unknown (>90d)",
		},
	}
}

func TestFormatAsTableWithOwner(t *testing.T) {
	volumes := getOwnerTestVolumes()
	buffer := &bytes.Buffer{}

	err := formatAsTable(volumes, calculateTotalCost(volumes), buffer, true)
	assert.NoError(t, err)

	output := buffer.String()
	assert.Contains(t, output, "CREATED BY")
	assert.Contains(t, output, "CREATED AT")
	assert.Contains(t, output, "DeployRole/alice")
	assert.Contains(t, output, "2026-03-01")
	assert.Contains(t, output, "unknown (>90d)")
	assert.Contains(t, output, "TOTAL")
	assert.Contains(t, output, "17.00")
}

func TestFormatAsTableEscapesTerminalControlCharacters(t *testing.T) {
	volumes := getOwnerTestVolumes()
	volumes[0].CreatedBy = "DeployRole/\x1b]52;c;payload\a\nforged\u202Etxt\U000E0001"
	buffer := &bytes.Buffer{}

	err := formatAsTable(volumes, calculateTotalCost(volumes), buffer, true)
	assert.NoError(t, err)

	output := buffer.String()
	assert.NotContains(t, output, "\x1b")
	assert.NotContains(t, output, "\a")
	assert.NotContains(t, output, "\u202E")
	assert.NotContains(t, output, "\U000E0001")
	assert.Contains(t, output, `DeployRole/\u001B]52;c;payload\u0007\u000Aforged\u202Etxt\U000E0001`)
}

func TestFormatAsCSVWithOwner(t *testing.T) {
	volumes := getOwnerTestVolumes()
	buffer := &bytes.Buffer{}

	err := formatAsCSV(volumes, calculateTotalCost(volumes), buffer, true)
	assert.NoError(t, err)

	output := buffer.String()
	assert.Contains(t, output, "Volume ID,Type,Size (GB),State,Created By,Created At,Monthly Cost ($)")
	assert.Contains(t, output, "vol-123,gp2,100,available,DeployRole/alice,2026-03-01,10.00")
	// A volume without a creation timestamp leaves the column empty.
	assert.Contains(t, output, "vol-456,io1,70,available,unknown (>90d),,7.00")
	assert.Contains(t, output, "Total,,,,,,17.00")
}

func TestFormatWithoutOwnerOmitsColumns(t *testing.T) {
	volumes := getOwnerTestVolumes()
	buffer := &bytes.Buffer{}

	err := FormatEBSOutputTo(volumes, CSVFormat, buffer, false)
	assert.NoError(t, err)

	output := buffer.String()
	assert.NotContains(t, output, "Created By")
	assert.NotContains(t, output, "DeployRole/alice")
	assert.Contains(t, output, "vol-123,gp2,100,available,10.00")
}

func TestFormatAsJSONIncludesOwner(t *testing.T) {
	volumes := getOwnerTestVolumes()
	buffer := &bytes.Buffer{}

	err := formatAsJSON(volumes, calculateTotalCost(volumes), buffer)
	assert.NoError(t, err)

	output := buffer.String()
	assert.Contains(t, output, `"createdBy": "DeployRole/alice"`)
	assert.Contains(t, output, `"createdAt": "2026-03-01T12:00:00Z"`)
}

func TestFormatAsJSONOmitsEmptyOwner(t *testing.T) {
	buffer := &bytes.Buffer{}

	err := formatAsJSON(getTestVolumes(), 17.0, buffer)
	assert.NoError(t, err)

	output := buffer.String()
	assert.NotContains(t, output, "createdBy")
	assert.NotContains(t, output, "createdAt")
}

func TestFormatCreatedAt(t *testing.T) {
	assert.Equal(t, "", formatCreatedAt(nil))

	ts := time.Date(2026, 3, 1, 23, 30, 0, 0, time.FixedZone("JST", 9*60*60))
	// Timestamps are normalized to UTC before formatting.
	assert.Equal(t, "2026-03-01", formatCreatedAt(&ts))
}
