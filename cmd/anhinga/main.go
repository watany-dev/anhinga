package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/watany-dev/anhinga/internal/aws"
	"github.com/watany-dev/anhinga/internal/output"
)

type volumeLoader func(aws.Options) ([]aws.EBSInfo, error)

type outputFormatter func([]aws.EBSInfo, output.FormatType, io.Writer, bool) error

func run(args []string, stdout, stderr io.Writer, loadVolumes volumeLoader, formatVolumes outputFormatter) error {
	flags := flag.NewFlagSet("anhinga", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.Usage = func() {
		_, _ = fmt.Fprintln(stderr, "Usage: anhinga [options]")
		flags.SetOutput(stderr)
		flags.PrintDefaults()
		flags.SetOutput(io.Discard)
	}

	var region, formatName string
	var showOwner bool
	flags.StringVar(&region, "region", "", "AWS region (uses the AWS SDK default when omitted)")
	flags.StringVar(&region, "r", "", "shorthand for --region")
	flags.StringVar(&formatName, "format", "table", "output format: table, csv, or json")
	flags.StringVar(&formatName, "f", "table", "shorthand for --format")
	flags.BoolVar(&showOwner, "show-owner", false, "resolve creators through CloudTrail")

	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected arguments: %s", strings.Join(flags.Args(), " "))
	}

	format := output.FormatType(strings.ToLower(formatName))
	if format != output.TableFormat && format != output.CSVFormat && format != output.JSONFormat {
		return errors.New("format must be either 'table', 'csv', or 'json'")
	}

	volumes, err := loadVolumes(aws.Options{
		Region:    region,
		ShowOwner: showOwner,
		OnWarning: func(message string) {
			_, _ = fmt.Fprintln(stderr, "warning:", message)
		},
	})
	if err != nil {
		return fmt.Errorf("failed to get EBS volumes: %w", err)
	}
	return formatVolumes(volumes, format, stdout, showOwner)
}

func main() {
	if err := run(os.Args[1:], os.Stdout, os.Stderr, aws.GetEBSVolumesWithOptions, output.FormatEBSOutputTo); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
