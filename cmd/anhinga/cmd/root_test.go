package cmd

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

func TestRootCommandRejectsInvalidFormatBeforeLoading(t *testing.T) {
	loaderCalled := false
	command := newRootCommand(
		func(aws.Options) ([]aws.EBSInfo, error) {
			loaderCalled = true
			return nil, nil
		},
		func([]aws.EBSInfo, output.FormatType, io.Writer, ...output.Option) error { return nil },
	)
	command.SetArgs([]string{"--format", "xml"})

	err := command.Execute()

	assert.ErrorContains(t, err, "format must be")
	assert.False(t, loaderCalled)
}

func TestRootCommandPassesOptionsWarningsAndOutput(t *testing.T) {
	var receivedOptions aws.Options
	var receivedFormat output.FormatType
	var receivedOutputOptions int
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	command := newRootCommand(
		func(opts aws.Options) ([]aws.EBSInfo, error) {
			receivedOptions = opts
			opts.OnWarning("lookup failed")
			return []aws.EBSInfo{{VolumeID: "vol-1"}}, nil
		},
		func(_ []aws.EBSInfo, format output.FormatType, writer io.Writer, opts ...output.Option) error {
			receivedFormat = format
			receivedOutputOptions = len(opts)
			_, err := io.WriteString(writer, "formatted\n")
			return err
		},
	)
	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetArgs([]string{"--region", "us-west-2", "--format", "CSV", "--show-owner"})

	err := command.Execute()

	require.NoError(t, err)
	assert.Equal(t, "us-west-2", receivedOptions.Region)
	assert.True(t, receivedOptions.ShowOwner)
	assert.NotNil(t, receivedOptions.OnWarning)
	assert.Equal(t, output.CSVFormat, receivedFormat)
	assert.Equal(t, 1, receivedOutputOptions)
	assert.Equal(t, "formatted\n", stdout.String())
	assert.Contains(t, stderr.String(), "warning: lookup failed")
}

func TestRootCommandWrapsLoaderErrors(t *testing.T) {
	loadError := errors.New("AWS unavailable")
	command := newRootCommand(
		func(aws.Options) ([]aws.EBSInfo, error) { return nil, loadError },
		func([]aws.EBSInfo, output.FormatType, io.Writer, ...output.Option) error { return nil },
	)
	command.SetArgs(nil)

	err := command.Execute()

	assert.ErrorIs(t, err, loadError)
	assert.ErrorContains(t, err, "failed to get EBS volumes")
}

func TestRootCommandRejectsPositionalArguments(t *testing.T) {
	loaderCalled := false
	command := newRootCommand(
		func(aws.Options) ([]aws.EBSInfo, error) {
			loaderCalled = true
			return nil, nil
		},
		func([]aws.EBSInfo, output.FormatType, io.Writer, ...output.Option) error { return nil },
	)
	command.SetArgs([]string{"unexpected"})

	err := command.Execute()

	assert.Error(t, err)
	assert.False(t, loaderCalled)
}
