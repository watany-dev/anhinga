package aws

import (
	"context"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// EBSInfo represents information about an EBS volume
type EBSInfo struct {
	VolumeID   string
	VolumeType string
	Size       int32
	State      string
	Cost       float64
}

// GetEBSVolumes retrieves all EBS volumes in the specified region
func GetEBSVolumes(region string) ([]EBSInfo, error) {
	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(context.TODO(), config.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("unable to load SDK config: %v", err)
	}

	// Create EC2 client
	client := ec2.NewFromConfig(cfg)

	// Describe volumes
	resp, err := client.DescribeVolumes(context.TODO(), &ec2.DescribeVolumesInput{})
	if err != nil {
		return nil, fmt.Errorf("failed to describe volumes: %v", err)
	}

	// Process volumes
	var volumesInfo []EBSInfo
	for _, volume := range resp.Volumes {
		cost := calculateVolumeCost(volume, region)
		
		volumesInfo = append(volumesInfo, EBSInfo{
			VolumeID:   *volume.VolumeId,
			VolumeType: string(volume.VolumeType),
			Size:       *volume.Size,
			State:      string(volume.State),
			Cost:       cost,
		})
	}

	return volumesInfo, nil
}

// calculateVolumeCost calculates the monthly cost of an EBS volume
func calculateVolumeCost(volume types.Volume, region string) float64 {
	// Pricing per GB-month varies by region and volume type
	// These are example prices, actual AWS pricing should be used in production
	var pricePerGB float64

	switch volume.VolumeType {
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
	return float64(*volume.Size) * pricePerGB
}