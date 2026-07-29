package aws

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudtrail"
	cttypes "github.com/aws/aws-sdk-go-v2/service/cloudtrail/types"
	"github.com/aws/smithy-go"
)

const (
	// CloudTrailRetention is how far back the CloudTrail event history can be
	// queried with LookupEvents. Volumes created before that are unresolvable
	// without a dedicated trail or CloudTrail Lake.
	CloudTrailRetention = 90 * 24 * time.Hour

	// lookupInterval is the minimum gap between two LookupEvents calls.
	// CloudTrail allows 2 requests per second per account per region.
	lookupInterval = 500 * time.Millisecond

	// lookupMaxRetries is the number of retries on throttling before giving up.
	lookupMaxRetries = 4

	// unknownOwner is reported when the creator could not be determined.
	unknownOwner = "unknown"
)

// ErrOutsideRetention is returned when a volume predates the CloudTrail event
// history window, so no lookup is even attempted.
var ErrOutsideRetention = errors.New("volume is older than the CloudTrail event history retention")

// ErrEventNotFound is returned when no CreateVolume event exists for a volume.
var ErrEventNotFound = errors.New("no CreateVolume event found")

// cloudTrailAPI is the subset of the CloudTrail client used here, kept as an
// interface so the resolver can be tested without network access.
type cloudTrailAPI interface {
	LookupEvents(ctx context.Context, params *cloudtrail.LookupEventsInput, optFns ...func(*cloudtrail.Options)) (*cloudtrail.LookupEventsOutput, error)
}

// OwnerResolver resolves the principal that created an EBS volume by looking
// up the corresponding CreateVolume event in the CloudTrail event history.
type OwnerResolver struct {
	client cloudTrailAPI

	// interval is the minimum gap enforced between two API calls.
	interval time.Duration

	// maxRetries bounds the exponential backoff on throttling errors.
	maxRetries int

	// now and sleep are injectable so tests do not depend on wall clock time.
	now   func() time.Time
	sleep func(time.Duration)

	lastCall time.Time
}

// NewOwnerResolver builds a resolver from an AWS config.
func NewOwnerResolver(cfg aws.Config) *OwnerResolver {
	return newOwnerResolver(cloudtrail.NewFromConfig(cfg))
}

func newOwnerResolver(client cloudTrailAPI) *OwnerResolver {
	return &OwnerResolver{
		client:     client,
		interval:   lookupInterval,
		maxRetries: lookupMaxRetries,
		now:        time.Now,
		sleep:      time.Sleep,
	}
}

// Resolve returns a human readable identifier of the principal that created
// the given volume. createdAt may be nil; when known it is used to skip the
// API call entirely for volumes outside the CloudTrail retention window.
func (r *OwnerResolver) Resolve(ctx context.Context, volumeID string, createdAt *time.Time) (string, error) {
	if r == nil || r.client == nil {
		return "", errors.New("owner resolver is not initialized")
	}

	windowStart := r.now().Add(-CloudTrailRetention)
	if createdAt != nil && createdAt.Before(windowStart) {
		return "", ErrOutsideRetention
	}

	start := windowStart
	end := r.now()
	if createdAt != nil {
		// Narrow the window around the known creation time to keep the
		// result set small and unambiguous.
		start = createdAt.Add(-1 * time.Hour)
		end = createdAt.Add(1 * time.Hour)
	}

	input := &cloudtrail.LookupEventsInput{
		LookupAttributes: []cttypes.LookupAttribute{
			{
				AttributeKey:   cttypes.LookupAttributeKeyResourceName,
				AttributeValue: aws.String(volumeID),
			},
		},
		StartTime:  aws.Time(start),
		EndTime:    aws.Time(end),
		MaxResults: aws.Int32(50),
	}

	resp, err := r.lookupWithRetry(ctx, input)
	if err != nil {
		return "", fmt.Errorf("failed to look up CloudTrail events for %s: %w", volumeID, err)
	}

	// Only one lookup attribute may be supplied per request, so CreateVolume
	// is filtered on the client side.
	for _, event := range resp.Events {
		if event.EventName == nil || *event.EventName != "CreateVolume" {
			continue
		}
		if event.CloudTrailEvent == nil {
			continue
		}
		creator, err := parseCreator(*event.CloudTrailEvent)
		if err != nil {
			return "", fmt.Errorf("failed to parse CloudTrail event for %s: %w", volumeID, err)
		}
		return creator, nil
	}

	return "", ErrEventNotFound
}

// lookupWithRetry calls LookupEvents while respecting the API rate limit and
// retrying throttled requests with exponential backoff.
func (r *OwnerResolver) lookupWithRetry(ctx context.Context, input *cloudtrail.LookupEventsInput) (*cloudtrail.LookupEventsOutput, error) {
	backoff := r.interval
	var lastErr error

	for attempt := 0; attempt <= r.maxRetries; attempt++ {
		if err := r.waitForSlot(ctx); err != nil {
			return nil, err
		}

		resp, err := r.client.LookupEvents(ctx, input)
		if err == nil {
			return resp, nil
		}
		lastErr = err

		if ctx.Err() != nil || !isThrottling(err) {
			return nil, err
		}

		if attempt < r.maxRetries {
			r.sleep(backoff)
			backoff *= 2
		}
	}

	return nil, lastErr
}

// waitForSlot spaces API calls out to stay under the LookupEvents rate limit.
func (r *OwnerResolver) waitForSlot(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !r.lastCall.IsZero() {
		if wait := r.interval - r.now().Sub(r.lastCall); wait > 0 {
			r.sleep(wait)
		}
	}
	r.lastCall = r.now()
	return nil
}

// IsAccessDenied reports whether the error means the caller lacks the
// cloudtrail:LookupEvents permission, in which case retrying every volume is
// pointless.
func IsAccessDenied(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return strings.Contains(code, "AccessDenied") ||
			strings.Contains(code, "UnauthorizedOperation") ||
			code == "AccessDeniedException"
	}
	return false
}

func isThrottling(err error) bool {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		code := apiErr.ErrorCode()
		return strings.Contains(code, "Throttling") ||
			strings.Contains(code, "TooManyRequests") ||
			strings.Contains(code, "RequestLimitExceeded")
	}
	return false
}

// ctEvent mirrors the fields of a raw CloudTrail event record that identify
// the calling principal.
type ctEvent struct {
	UserIdentity struct {
		Type           string `json:"type"`
		ARN            string `json:"arn"`
		UserName       string `json:"userName"`
		PrincipalID    string `json:"principalId"`
		InvokedBy      string `json:"invokedBy"`
		AccountID      string `json:"accountId"`
		SessionContext struct {
			SessionIssuer struct {
				Type     string `json:"type"`
				UserName string `json:"userName"`
				ARN      string `json:"arn"`
			} `json:"sessionIssuer"`
		} `json:"sessionContext"`
	} `json:"userIdentity"`
}

// parseCreator turns the userIdentity block of a CloudTrail event into a short
// human readable principal name.
func parseCreator(rawEvent string) (string, error) {
	var event ctEvent
	if err := json.Unmarshal([]byte(rawEvent), &event); err != nil {
		return "", fmt.Errorf("invalid CloudTrail event JSON: %w", err)
	}

	identity := event.UserIdentity

	switch identity.Type {
	case "IAMUser":
		if identity.UserName != "" {
			return identity.UserName, nil
		}
	case "AssumedRole":
		// principalId is "AROAEXAMPLE:session-name"; the session name is
		// usually the actual human or workload behind the role.
		role := identity.SessionContext.SessionIssuer.UserName
		session := ""
		if idx := strings.LastIndex(identity.PrincipalID, ":"); idx >= 0 {
			session = identity.PrincipalID[idx+1:]
		}
		switch {
		case role != "" && session != "":
			return role + "/" + session, nil
		case role != "":
			return role, nil
		case session != "":
			return session, nil
		}
	case "Root":
		return "root", nil
	case "AWSService":
		if identity.InvokedBy != "" {
			return identity.InvokedBy, nil
		}
	case "AWSAccount":
		if identity.AccountID != "" {
			return "account:" + identity.AccountID, nil
		}
	}

	// Fall back to whatever identifies the caller at all.
	switch {
	case identity.ARN != "":
		return identity.ARN, nil
	case identity.InvokedBy != "":
		return identity.InvokedBy, nil
	case identity.PrincipalID != "":
		return identity.PrincipalID, nil
	case identity.Type != "":
		return identity.Type, nil
	}

	return unknownOwner, nil
}
