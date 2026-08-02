package main

import (
	"bytes"
	"errors"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/watany-dev/anhinga/internal/aws"
	"github.com/watany-dev/anhinga/internal/output"
)

func TestRunRejectsInvalidFormatBeforeLoading(t *testing.T) {
	loaderCalled := false
	err := run([]string{"--format", "xml"}, io.Discard, io.Discard,
		func(aws.Options) ([]aws.EBSInfo, error) {
			loaderCalled = true
			return nil, nil
		},
		func([]aws.EBSInfo, output.FormatType, io.Writer, bool) error { return nil },
	)

	assert.ErrorContains(t, err, "format must be")
	assert.False(t, loaderCalled)
}

func TestRunPassesOptionsWarningsAndOutput(t *testing.T) {
	var receivedOptions aws.Options
	var receivedFormat output.FormatType
	var receivedShowOwner bool
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}

	err := run([]string{"-r", "us-west-2", "-f", "CSV", "--show-owner"}, stdout, stderr,
		func(opts aws.Options) ([]aws.EBSInfo, error) {
			receivedOptions = opts
			opts.OnWarning("lookup failed")
			return []aws.EBSInfo{{VolumeID: "vol-1"}}, nil
		},
		func(_ []aws.EBSInfo, format output.FormatType, writer io.Writer, showOwner bool) error {
			receivedFormat = format
			receivedShowOwner = showOwner
			_, err := io.WriteString(writer, "formatted\n")
			return err
		},
	)

	require.NoError(t, err)
	assert.Equal(t, "us-west-2", receivedOptions.Region)
	assert.True(t, receivedOptions.ShowOwner)
	assert.NotNil(t, receivedOptions.OnWarning)
	assert.Equal(t, output.CSVFormat, receivedFormat)
	assert.True(t, receivedShowOwner)
	assert.Equal(t, "formatted\n", stdout.String())
	assert.Contains(t, stderr.String(), "warning: lookup failed")
}

func TestRunWrapsLoaderErrors(t *testing.T) {
	loadError := errors.New("AWS unavailable")
	err := run(nil, io.Discard, io.Discard,
		func(aws.Options) ([]aws.EBSInfo, error) { return nil, loadError },
		func([]aws.EBSInfo, output.FormatType, io.Writer, bool) error { return nil },
	)

	assert.ErrorIs(t, err, loadError)
	assert.ErrorContains(t, err, "failed to get EBS volumes")
}

func TestRunRejectsPositionalArguments(t *testing.T) {
	loaderCalled := false
	err := run([]string{"unexpected"}, io.Discard, io.Discard,
		func(aws.Options) ([]aws.EBSInfo, error) {
			loaderCalled = true
			return nil, nil
		},
		func([]aws.EBSInfo, output.FormatType, io.Writer, bool) error { return nil },
	)

	assert.Error(t, err)
	assert.False(t, loaderCalled)
}

func TestRunHelp(t *testing.T) {
	stderr := &bytes.Buffer{}
	err := run([]string{"--help"}, io.Discard, stderr,
		func(aws.Options) ([]aws.EBSInfo, error) { return nil, nil },
		func([]aws.EBSInfo, output.FormatType, io.Writer, bool) error { return nil },
	)

	require.NoError(t, err)
	assert.Contains(t, stderr.String(), "Usage: anhinga")
}
