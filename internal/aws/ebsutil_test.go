package aws

import (
	"context"
	"errors"
	"testing"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockEC2 struct {
	responses []*ec2.DescribeVolumesOutput
	errs      []error
	inputs    []*ec2.DescribeVolumesInput
}

func (m *mockEC2) DescribeVolumes(_ context.Context, input *ec2.DescribeVolumesInput, _ ...func(*ec2.Options)) (*ec2.DescribeVolumesOutput, error) {
	inputCopy := *input
	m.inputs = append(m.inputs, &inputCopy)
	index := len(m.inputs) - 1
	if index < len(m.errs) && m.errs[index] != nil {
		return nil, m.errs[index]
	}
	if index < len(m.responses) {
		return m.responses[index], nil
	}
	return &ec2.DescribeVolumesOutput{}, nil
}

func testVolume(id string, size int32) types.Volume {
	return types.Volume{
		VolumeId:   awssdk.String(id),
		VolumeType: types.VolumeTypeGp3,
		Size:       awssdk.Int32(size),
		State:      types.VolumeStateAvailable,
	}
}

func TestDescribeEBSVolumesPaginates(t *testing.T) {
	client := &mockEC2{responses: []*ec2.DescribeVolumesOutput{
		{Volumes: []types.Volume{testVolume("vol-1", 100)}, NextToken: awssdk.String("page-2")},
		{Volumes: []types.Volume{testVolume("vol-2", 200)}},
	}}

	volumes, err := describeEBSVolumes(context.Background(), client, "us-east-1")

	require.NoError(t, err)
	require.Len(t, volumes, 2)
	assert.Equal(t, "vol-1", volumes[0].VolumeID)
	assert.Equal(t, 8.0, volumes[0].Cost)
	assert.Equal(t, "vol-2", volumes[1].VolumeID)
	assert.Equal(t, 16.0, volumes[1].Cost)
	require.Len(t, client.inputs, 2)
	assert.Nil(t, client.inputs[0].NextToken)
	assert.Equal(t, int32(500), awssdk.ToInt32(client.inputs[0].MaxResults))
	assert.Equal(t, "page-2", awssdk.ToString(client.inputs[1].NextToken))
}

func TestDescribeEBSVolumesRejectsMalformedVolumes(t *testing.T) {
	tests := []struct {
		name   string
		volume types.Volume
	}{
		{name: "missing volume ID", volume: types.Volume{Size: awssdk.Int32(1)}},
		{name: "empty volume ID", volume: types.Volume{VolumeId: awssdk.String(""), Size: awssdk.Int32(1)}},
		{name: "missing size", volume: types.Volume{VolumeId: awssdk.String("vol-1")}},
		{name: "zero size", volume: testVolume("vol-1", 0)},
		{name: "negative size", volume: testVolume("vol-1", -1)},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			client := &mockEC2{responses: []*ec2.DescribeVolumesOutput{{Volumes: []types.Volume{tc.volume}}}}

			_, err := describeEBSVolumes(context.Background(), client, "us-east-1")

			assert.Error(t, err)
			assert.Contains(t, err.Error(), "page 1 volume 1")
		})
	}
}

func TestDescribeEBSVolumesRejectsNilResponse(t *testing.T) {
	client := &mockEC2{responses: []*ec2.DescribeVolumesOutput{nil}}

	_, err := describeEBSVolumes(context.Background(), client, "us-east-1")

	assert.ErrorContains(t, err, "nil response")
}

func TestDescribeEBSVolumesStopsOnDuplicateToken(t *testing.T) {
	client := &mockEC2{responses: []*ec2.DescribeVolumesOutput{
		{NextToken: awssdk.String("duplicate")},
		{NextToken: awssdk.String("duplicate")},
	}}

	_, err := describeEBSVolumes(context.Background(), client, "us-east-1")

	assert.ErrorContains(t, err, "duplicate pagination token")
	assert.Len(t, client.inputs, 2)
}

func TestDescribeEBSVolumesWrapsPageErrors(t *testing.T) {
	pageError := errors.New("service unavailable")
	client := &mockEC2{
		responses: []*ec2.DescribeVolumesOutput{{NextToken: awssdk.String("page-2")}},
		errs:      []error{nil, pageError},
	}

	_, err := describeEBSVolumes(context.Background(), client, "us-east-1")

	assert.ErrorIs(t, err, pageError)
	assert.ErrorContains(t, err, "page 2")
}
