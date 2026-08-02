package aws

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

const (
	// describeTimeout bounds the DescribeVolumes call.
	describeTimeout = 30 * time.Second

	// ownerTimeoutPerVolume is budgeted per volume when resolving creators,
	// since the CloudTrail lookups are rate limited and run sequentially.
	ownerTimeoutPerVolume = 3 * time.Second

	// ownerTimeoutMin is the floor for the owner resolution phase.
	ownerTimeoutMin = 60 * time.Second

	// ownerUnknownOutsideRetention labels volumes created before the
	// CloudTrail event history window.
	ownerUnknownOutsideRetention = "unknown (>90d)"
)

// EBSInfo represents information about an EBS volume
type EBSInfo struct {
	VolumeID   string     `json:"volumeId"`
	VolumeType string     `json:"volumeType"`
	Size       int32      `json:"size"`
	State      string     `json:"state"`
	Cost       float64    `json:"cost"`
	CreatedAt  *time.Time `json:"createdAt,omitempty"`
	CreatedBy  string     `json:"createdBy,omitempty"`
}

// Options controls how volume information is collected.
type Options struct {
	// Region is the AWS region to query. Empty means the SDK default.
	Region string

	// ShowOwner enables CloudTrail lookups to resolve who created each
	// volume. It costs one rate limited API call per volume, so it is opt-in.
	ShowOwner bool

	// OnWarning, when set, receives non-fatal messages such as individual
	// CloudTrail lookups that failed.
	OnWarning func(string)
}

func (o Options) warn(format string, args ...interface{}) {
	if o.OnWarning != nil {
		o.OnWarning(fmt.Sprintf(format, args...))
	}
}

// GetEBSVolumes retrieves all EBS volumes in the specified region
func GetEBSVolumes(region string) ([]EBSInfo, error) {
	return GetEBSVolumesWithOptions(Options{Region: region})
}

// GetEBSVolumesWithOptions retrieves all EBS volumes, optionally resolving the
// principal that created each of them via CloudTrail.
func GetEBSVolumesWithOptions(opts Options) ([]EBSInfo, error) {
	// Create context with timeout for AWS operations
	ctx, cancel := context.WithTimeout(context.Background(), describeTimeout)
	defer cancel()

	// Load AWS configuration
	var cfg aws.Config
	var err error

	if opts.Region != "" {
		cfg, err = config.LoadDefaultConfig(ctx, config.WithRegion(opts.Region))
	} else {
		cfg, err = config.LoadDefaultConfig(ctx)
	}
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %w", err)
	}

	// Create EC2 client
	client := ec2.NewFromConfig(cfg)

	volumesInfo, err := describeEBSVolumes(ctx, client, cfg.Region)
	if err != nil {
		return nil, fmt.Errorf("failed to describe volumes: %w", err)
	}

	if !opts.ShowOwner || len(volumesInfo) == 0 {
		return volumesInfo, nil
	}

	if err := resolveOwners(NewOwnerResolver(cfg), volumesInfo, opts); err != nil {
		return nil, err
	}

	return volumesInfo, nil
}

// describeEBSVolumes retrieves every response page and validates the required
// fields before converting SDK values into the application's data model.
func describeEBSVolumes(ctx context.Context, client ec2.DescribeVolumesAPIClient, region string) ([]EBSInfo, error) {
	var volumes []EBSInfo
	var nextToken *string
	seenTokens := make(map[string]struct{})

	for page := 1; ; page++ {
		response, err := client.DescribeVolumes(ctx, &ec2.DescribeVolumesInput{
			MaxResults: aws.Int32(500),
			NextToken:  nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		if response == nil {
			return nil, fmt.Errorf("page %d: EC2 returned a nil response", page)
		}

		for index, volume := range response.Volumes {
			info, err := ebsInfoFromVolume(volume, region)
			if err != nil {
				return nil, fmt.Errorf("page %d volume %d: %w", page, index+1, err)
			}
			volumes = append(volumes, info)
		}

		token := aws.ToString(response.NextToken)
		if token == "" {
			break
		}
		if _, exists := seenTokens[token]; exists {
			return nil, fmt.Errorf("page %d: duplicate pagination token %q", page, token)
		}
		seenTokens[token] = struct{}{}
		nextToken = aws.String(token)
	}

	return volumes, nil
}

func ebsInfoFromVolume(volume types.Volume, region string) (EBSInfo, error) {
	volumeID := aws.ToString(volume.VolumeId)
	if volumeID == "" {
		return EBSInfo{}, errors.New("missing volume ID")
	}
	if volume.Size == nil {
		return EBSInfo{}, fmt.Errorf("volume %s has no size", volumeID)
	}
	if *volume.Size <= 0 {
		return EBSInfo{}, fmt.Errorf("volume %s has invalid size %d", volumeID, *volume.Size)
	}

	return EBSInfo{
		VolumeID:   volumeID,
		VolumeType: string(volume.VolumeType),
		Size:       *volume.Size,
		State:      string(volume.State),
		Cost:       calculateVolumeCost(volume.VolumeType, *volume.Size, region),
		CreatedAt:  volume.CreateTime,
	}, nil
}

// resolveOwners fills in the CreatedBy field of every volume. Individual
// failures degrade to "unknown"; only a permission error aborts the whole
// phase, since it would fail identically for every remaining volume.
func resolveOwners(resolver *OwnerResolver, volumes []EBSInfo, opts Options) error {
	timeout := time.Duration(len(volumes)) * ownerTimeoutPerVolume
	if timeout < ownerTimeoutMin {
		timeout = ownerTimeoutMin
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	for i := range volumes {
		creator, err := resolver.Resolve(ctx, volumes[i].VolumeID, volumes[i].CreatedAt)
		switch {
		case err == nil:
			volumes[i].CreatedBy = creator
		case errors.Is(err, ErrOutsideRetention):
			volumes[i].CreatedBy = ownerUnknownOutsideRetention
		case errors.Is(err, ErrEventNotFound):
			volumes[i].CreatedBy = unknownOwner
		case IsAccessDenied(err):
			return fmt.Errorf("cloudtrail:LookupEvents is required for --show-owner: %w", err)
		default:
			volumes[i].CreatedBy = unknownOwner
			opts.warn("could not resolve creator of %s: %v", volumes[i].VolumeID, err)
		}
	}

	return nil
}

// calculateVolumeCost calculates the monthly cost of an EBS volume
func calculateVolumeCost(volumeType types.VolumeType, size int32, region string) float64 {
	// Pricing per GB-month varies by region and volume type
	// These are example prices, actual AWS pricing should be used in production
	var pricePerGB float64

	switch volumeType {
	case types.VolumeTypeGp2:
		pricePerGB = 0.10
	case types.VolumeTypeGp3:
		pricePerGB = 0.08
	case types.VolumeTypeIo1:
		pricePerGB = 0.125
	case types.VolumeTypeIo2:
		pricePerGB = 0.125
	case types.VolumeTypeSt1:
		pricePerGB = 0.045
	case types.VolumeTypeSc1:
		pricePerGB = 0.025
	case types.VolumeTypeStandard:
		pricePerGB = 0.05
	default:
		pricePerGB = 0.10
	}

	// Adjust price based on region (simplified approach)
	// In production, you'd use the AWS Pricing API or a pricing database
	if region != "us-east-1" {
		// Slightly higher prices for regions other than us-east-1
		pricePerGB *= 1.1
	}

	// Calculate monthly cost
	return float64(size) * pricePerGB
}
