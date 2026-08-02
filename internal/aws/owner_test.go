package aws

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockCloudTrail returns canned responses and records the requests it received.
type mockCloudTrail struct {
	responses []*cloudtrail.LookupEventsOutput
	errs      []error
	inputs    []*cloudtrail.LookupEventsInput
	calls     int
}

func (m *mockCloudTrail) LookupEvents(_ context.Context, params *cloudtrail.LookupEventsInput, _ ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error) {
	paramsCopy := *params
	m.inputs = append(m.inputs, &paramsCopy)
	idx := m.calls
	m.calls++

	if idx < len(m.errs) && m.errs[idx] != nil {
		return nil, m.errs[idx]
	}
	if idx < len(m.responses) {
		return m.responses[idx], nil
	}
	return &cloudtrail.LookupEventsOutput{}, nil
}

// newTestResolver wires a resolver with a mock client and a fake clock so the
// rate limiting never actually sleeps.
func newTestResolver(client cloudTrailAPI) *OwnerResolver {
	r := newOwnerResolver(client)
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	r.now = func() time.Time { return now }
	r.wait = func(_ context.Context, d time.Duration) error {
		now = now.Add(d)
		return nil
	}
	return r
}

func createVolumeEvent(rawEvent string) *cloudtrail.LookupEventsOutput {
	return &cloudtrail.LookupEventsOutput{
		Events: []cttypes.Event{
			{
				EventName:       aws.String("CreateVolume"),
				CloudTrailEvent: aws.String(rawEvent),
			},
		},
	}
}

const assumedRoleEvent = `{
  "eventName": "CreateVolume",
  "userIdentity": {
    "type": "AssumedRole",
    "principalId": "AROAEXAMPLE:alice",
    "arn": "arn:aws:sts::123456789012:assumed-role/DeployRole/alice",
    "sessionContext": {"sessionIssuer": {"type": "Role", "userName": "DeployRole"}}
  }
}`

func TestResolveAssumedRole(t *testing.T) {
	client := &mockCloudTrail{responses: []*cloudtrail.LookupEventsOutput{createVolumeEvent(assumedRoleEvent)}}
	resolver := newTestResolver(client)

	createdAt := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	creator, err := resolver.Resolve(context.Background(), "vol-123", &createdAt)

	require.NoError(t, err)
	assert.Equal(t, "DeployRole/alice", creator)

	require.Len(t, client.inputs, 1)
	input := client.inputs[0]
	require.Len(t, input.LookupAttributes, 1)
	assert.Equal(t, cttypes.LookupAttributeKeyResourceName, input.LookupAttributes[0].AttributeKey)
	assert.Equal(t, "vol-123", *input.LookupAttributes[0].AttributeValue)
	// The lookup window is narrowed around the known creation time.
	assert.True(t, input.StartTime.Before(createdAt))
	assert.True(t, input.EndTime.After(createdAt))
}

func TestResolveSkipsVolumesOutsideRetention(t *testing.T) {
	client := &mockCloudTrail{}
	resolver := newTestResolver(client)

	createdAt := resolver.now().Add(-CloudTrailRetention - time.Hour)
	_, err := resolver.Resolve(context.Background(), "vol-old", &createdAt)

	assert.ErrorIs(t, err, ErrOutsideRetention)
	assert.Zero(t, client.calls, "no API call should be made for volumes outside the retention window")
}

func TestResolveWithoutCreationTimeUsesFullWindow(t *testing.T) {
	client := &mockCloudTrail{responses: []*cloudtrail.LookupEventsOutput{createVolumeEvent(assumedRoleEvent)}}
	resolver := newTestResolver(client)

	creator, err := resolver.Resolve(context.Background(), "vol-123", nil)

	require.NoError(t, err)
	assert.Equal(t, "DeployRole/alice", creator)
	require.Len(t, client.inputs, 1)
	assert.Equal(t, resolver.now().Add(-CloudTrailRetention), *client.inputs[0].StartTime)
}

func TestResolveIgnoresOtherEventNames(t *testing.T) {
	client := &mockCloudTrail{responses: []*cloudtrail.LookupEventsOutput{
		{
			Events: []cttypes.Event{
				{EventName: aws.String("AttachVolume"), CloudTrailEvent: aws.String(assumedRoleEvent)},
				{EventName: aws.String("CreateVolume"), CloudTrailEvent: aws.String(`{"userIdentity":{"type":"IAMUser","userName":"bob"}}`)},
			},
		},
	}}
	resolver := newTestResolver(client)

	creator, err := resolver.Resolve(context.Background(), "vol-123", nil)

	require.NoError(t, err)
	assert.Equal(t, "bob", creator)
}

func TestResolveEventNotFound(t *testing.T) {
	client := &mockCloudTrail{responses: []*cloudtrail.LookupEventsOutput{{}}}
	resolver := newTestResolver(client)

	_, err := resolver.Resolve(context.Background(), "vol-123", nil)

	assert.ErrorIs(t, err, ErrEventNotFound)
}

func TestResolveFindsCreateEventOnLaterPage(t *testing.T) {
	client := &mockCloudTrail{responses: []*cloudtrail.LookupEventsOutput{
		{
			Events:    []cttypes.Event{{EventName: aws.String("AttachVolume")}},
			NextToken: aws.String("page-2"),
		},
		createVolumeEvent(assumedRoleEvent),
	}}
	resolver := newTestResolver(client)

	creator, err := resolver.Resolve(context.Background(), "vol-123", nil)

	require.NoError(t, err)
	assert.Equal(t, "DeployRole/alice", creator)
	require.Len(t, client.inputs, 2)
	assert.Nil(t, client.inputs[0].NextToken)
	assert.Equal(t, "page-2", aws.ToString(client.inputs[1].NextToken))
}

func TestResolveRejectsNilResponse(t *testing.T) {
	client := &mockCloudTrail{responses: []*cloudtrail.LookupEventsOutput{nil}}
	resolver := newTestResolver(client)

	_, err := resolver.Resolve(context.Background(), "vol-123", nil)

	assert.ErrorContains(t, err, "nil response")
}

func TestResolveStopsOnDuplicateToken(t *testing.T) {
	client := &mockCloudTrail{responses: []*cloudtrail.LookupEventsOutput{
		{NextToken: aws.String("duplicate")},
		{NextToken: aws.String("duplicate")},
	}}
	resolver := newTestResolver(client)

	_, err := resolver.Resolve(context.Background(), "vol-123", nil)

	assert.ErrorContains(t, err, "duplicate pagination token")
	assert.Equal(t, 2, client.calls)
}

func TestResolveRateLimitsConsecutiveCalls(t *testing.T) {
	client := &mockCloudTrail{}
	resolver := newTestResolver(client)

	var slept []time.Duration
	now := time.Date(2026, 7, 29, 0, 0, 0, 0, time.UTC)
	resolver.now = func() time.Time { return now }
	resolver.wait = func(_ context.Context, d time.Duration) error {
		slept = append(slept, d)
		now = now.Add(d)
		return nil
	}

	_, _ = resolver.Resolve(context.Background(), "vol-1", nil)
	_, _ = resolver.Resolve(context.Background(), "vol-2", nil)

	assert.Equal(t, []time.Duration{lookupInterval}, slept,
		"the second call should wait for the rate limit window")
}

func TestResolveRespectsCanceledContext(t *testing.T) {
	client := &mockCloudTrail{}
	resolver := newTestResolver(client)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := resolver.Resolve(ctx, "vol-123", nil)

	assert.ErrorIs(t, err, context.Canceled)
	assert.Zero(t, client.calls)
}

func TestWaitForContextStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- waitForContext(ctx, time.Hour)
	}()
	cancel()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.Canceled)
	case <-time.After(time.Second):
		t.Fatal("wait did not stop after context cancellation")
	}
}

func TestResolveUninitialized(t *testing.T) {
	var resolver *OwnerResolver
	_, err := resolver.Resolve(context.Background(), "vol-123", nil)
	assert.Error(t, err)
}

func TestParseCreator(t *testing.T) {
	tests := []struct {
		name     string
		event    string
		expected string
	}{
		{
			name:     "IAM user",
			event:    `{"userIdentity":{"type":"IAMUser","userName":"alice","arn":"arn:aws:iam::123456789012:user/alice"}}`,
			expected: "alice",
		},
		{
			name:     "assumed role with session name",
			event:    assumedRoleEvent,
			expected: "DeployRole/alice",
		},
		{
			name:     "assumed role without session issuer",
			event:    `{"userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE:ci-runner"}}`,
			expected: "ci-runner",
		},
		{
			name:     "assumed role without session name",
			event:    `{"userIdentity":{"type":"AssumedRole","principalId":"AROAEXAMPLE","sessionContext":{"sessionIssuer":{"userName":"DeployRole"}}}}`,
			expected: "DeployRole",
		},
		{
			name:     "root user",
			event:    `{"userIdentity":{"type":"Root","arn":"arn:aws:iam::123456789012:root"}}`,
			expected: "root",
		},
		{
			name:     "aws service",
			event:    `{"userIdentity":{"type":"AWSService","invokedBy":"autoscaling.amazonaws.com"}}`,
			expected: "autoscaling.amazonaws.com",
		},
		{
			name:     "another account",
			event:    `{"userIdentity":{"type":"AWSAccount","accountId":"210987654321"}}`,
			expected: "account:210987654321",
		},
		{
			name:     "unknown type falls back to arn",
			event:    `{"userIdentity":{"type":"FederatedUser","arn":"arn:aws:sts::123456789012:federated-user/carol"}}`,
			expected: "arn:aws:sts::123456789012:federated-user/carol",
		},
		{
			name:     "iam user without user name falls back to arn",
			event:    `{"userIdentity":{"type":"IAMUser","arn":"arn:aws:iam::123456789012:user/dave"}}`,
			expected: "arn:aws:iam::123456789012:user/dave",
		},
		{
			name:     "empty identity",
			event:    `{"userIdentity":{}}`,
			expected: unknownOwner,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			creator, err := parseCreator(tc.event)
			require.NoError(t, err)
			assert.Equal(t, tc.expected, creator)
		})
	}
}

func TestParseCreatorInvalidJSON(t *testing.T) {
	_, err := parseCreator("not json")
	assert.Error(t, err)
}

func TestIsAccessDenied(t *testing.T) {
	assert.True(t, IsAccessDenied(&smithy.GenericAPIError{Code: "AccessDeniedException"}))
	assert.True(t, IsAccessDenied(&smithy.GenericAPIError{Code: "UnauthorizedOperation"}))
	assert.False(t, IsAccessDenied(&smithy.GenericAPIError{Code: "ThrottlingException"}))
	assert.False(t, IsAccessDenied(errors.New("boom")))
	assert.False(t, IsAccessDenied(nil))
}

func TestResolveOwners(t *testing.T) {
	oldCreation := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	recentCreation := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)

	volumes := []EBSInfo{
		{VolumeID: "vol-recent", CreatedAt: &recentCreation},
		{VolumeID: "vol-old", CreatedAt: &oldCreation},
		{VolumeID: "vol-missing", CreatedAt: &recentCreation},
		{VolumeID: "vol-error", CreatedAt: &recentCreation},
	}

	client := &mockCloudTrail{
		responses: []*cloudtrail.LookupEventsOutput{
			createVolumeEvent(assumedRoleEvent),
			{}, // vol-missing: no CreateVolume event in the window
			nil,
		},
		errs: []error{nil, nil, &smithy.GenericAPIError{Code: "InternalError"}},
	}
	resolver := newTestResolver(client)
	resolver.wait = func(context.Context, time.Duration) error { return nil }

	var warnings []string
	opts := Options{ShowOwner: true, OnWarning: func(msg string) { warnings = append(warnings, msg) }}

	require.NoError(t, resolveOwners(resolver, volumes, opts))

	assert.Equal(t, "DeployRole/alice", volumes[0].CreatedBy)
	assert.Equal(t, ownerUnknownOutsideRetention, volumes[1].CreatedBy)
	assert.Equal(t, unknownOwner, volumes[2].CreatedBy)
	assert.Equal(t, unknownOwner, volumes[3].CreatedBy)
	assert.Len(t, warnings, 1)
	assert.Contains(t, warnings[0], "vol-error")
}

func TestResolveOwnersAbortsOnAccessDenied(t *testing.T) {
	created := time.Date(2026, 7, 20, 0, 0, 0, 0, time.UTC)
	volumes := []EBSInfo{
		{VolumeID: "vol-1", CreatedAt: &created},
		{VolumeID: "vol-2", CreatedAt: &created},
	}

	client := &mockCloudTrail{errs: []error{&smithy.GenericAPIError{Code: "AccessDeniedException"}}}
	resolver := newTestResolver(client)

	err := resolveOwners(resolver, volumes, Options{ShowOwner: true})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "cloudtrail:LookupEvents")
	assert.Equal(t, 1, client.calls, "a permission error should stop the remaining lookups")
}
