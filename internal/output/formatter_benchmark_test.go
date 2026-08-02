package output

import (
	"io"
	"strconv"
	"testing"

	"github.com/anhinga/anhinga/internal/aws"
)

const benchmarkVolumeCount = 10_000

func benchmarkVolumes() []aws.EBSInfo {
	volumes := make([]aws.EBSInfo, benchmarkVolumeCount)
	for i := range volumes {
		volumes[i] = aws.EBSInfo{
			VolumeID:   "vol-" + strconv.Itoa(i),
			VolumeType: "gp3",
			Size:       int32(100 + i%1_000),
			State:      "available",
			Cost:       float64(100+i%1_000) * 0.08,
		}
	}
	return volumes
}

func benchmarkFormat(b *testing.B, format FormatType) {
	volumes := benchmarkVolumes()
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := FormatEBSOutputTo(volumes, format, io.Discard); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkFormatTable10K(b *testing.B) {
	benchmarkFormat(b, TableFormat)
}

func BenchmarkFormatCSV10K(b *testing.B) {
	benchmarkFormat(b, CSVFormat)
}

func BenchmarkFormatJSON10K(b *testing.B) {
	benchmarkFormat(b, JSONFormat)
}
