# Anhinga

Anhinga is a Go CLI tool that lists AWS EBS volumes and estimates their monthly
storage costs based on volume type, size, and region.

## Features

- List all EBS volumes in a specified AWS region
- Estimate the monthly storage cost for each EBS volume
- Display results in table, CSV, or JSON format
- Show total cost of all EBS volumes

## Installation

```bash
curl -fsSL https://raw.githubusercontent.com/watany-dev/anhinga/main/install.sh | sh
anhinga -h
```

The installer downloads the release checksum and verifies the archive before
extracting the binary. Set `ANHINGA_INSTALL_DIR` to install somewhere other
than `/usr/local/bin`.

Or build from source:

```bash
git clone https://github.com/watany-dev/anhinga.git
cd anhinga
go build -o anhinga ./cmd/anhinga
```

## Usage

```bash
# Display EBS volumes in table format (default)
anhinga -r us-east-1

# Display EBS volumes in CSV format
anhinga -r us-east-1 -f csv

# Display EBS volumes in JSON format
anhinga -r us-east-1 -f json

# Show who created each volume (CloudTrail lookup)
anhinga -r us-east-1 --show-owner
```

### Flags

- `-r, --region` (optional): AWS region to query; the AWS SDK default is used when omitted
- `-f, --format`: Output format, either 'table', 'csv', or 'json' (default is 'table')
- `--show-owner`: Resolve who created each volume via CloudTrail (off by default)

## Identifying Who Created a Volume

`DescribeVolumes` does not report a creator, so `--show-owner` looks up the
`CreateVolume` event for each volume in the CloudTrail event history and reports
the calling principal (`DeployRole/alice`, `alice`, `root`,
`autoscaling.amazonaws.com`, ...).

Keep the following in mind before turning it on:

- **Extra permission**: the caller needs `cloudtrail:LookupEvents` in addition to
  `ec2:DescribeVolumes`. Without it the command fails with an explicit message
  instead of silently reporting every volume as unknown.
- **90 day window**: the CloudTrail event history only retains 90 days. Volumes
  created before that are reported as `unknown (>90d)` without an API call being
  made. For older volumes, query a dedicated trail with Athena or CloudTrail Lake.
- **Rate limited**: `LookupEvents` allows 2 requests per second, so lookups run
  sequentially with 500ms spacing and exponential backoff on throttling. Expect
  roughly half a second per volume; this is why the flag is opt-in.
- **Unknown values**: a volume whose `CreateVolume` event cannot be found (for
  example one restored outside the window) is reported as `unknown`. Lookup
  failures for individual volumes are printed as warnings on stderr and do not
  abort the run.

## AWS Authentication

Anhinga uses the AWS SDK for Go and follows the standard AWS authentication methods:

1. Environment variables
2. Shared credentials file (~/.aws/credentials)
3. IAM roles for EC2/ECS

Ensure your AWS credentials are properly configured before using this tool.

## Cost estimate limitations

The reported amount is an estimate based on built-in per-GB-month rates. It is
not an AWS bill or a quote from the AWS Pricing API. Provisioned IOPS,
throughput, snapshots, taxes, discounts, and other charges are not included.

## Example Output

### Table Format

```
VOLUME ID    TYPE  SIZE (GB)  STATE   MONTHLY COST ($)
vol-12345678  gp2   100        in-use  10.00
vol-87654321  io1   50         in-use  6.25
vol-11223344  gp3   500        in-use  40.00
                               TOTAL   56.25

Total EBS Monthly Cost: $56.25
```

### Table Format with `--show-owner`

```
VOLUME ID    TYPE  SIZE (GB)  STATE      CREATED BY                  CREATED AT  MONTHLY COST ($)
vol-12345678  gp2   100        in-use     DeployRole/alice            2026-06-14  10.00
vol-87654321  io1   50         available  unknown (>90d)                          6.25
vol-11223344  gp3   500        in-use     autoscaling.amazonaws.com  2026-07-02  40.00
                                                                      TOTAL       56.25

Total EBS Monthly Cost: $56.25
```

### CSV Format

```
Volume ID,Type,Size (GB),State,Monthly Cost ($)
vol-12345678,gp2,100,in-use,10.00
vol-87654321,io1,50,in-use,6.25
vol-11223344,gp3,500,in-use,40.00
Total,,,,56.25
```

### JSON Format

```json
{
  "volumes": [
    {
      "volumeId": "vol-12345678",
      "volumeType": "gp2",
      "size": 100,
      "state": "in-use",
      "cost": 10.00,
      "createdAt": "2026-06-14T10:30:00Z"
    },
    {
      "volumeId": "vol-87654321",
      "volumeType": "io1",
      "size": 50,
      "state": "in-use",
      "cost": 6.25,
      "createdAt": "2026-05-10T08:15:00Z"
    },
    {
      "volumeId": "vol-11223344",
      "volumeType": "gp3",
      "size": 500,
      "state": "in-use",
      "cost": 40.00,
      "createdAt": "2026-07-02T14:45:00Z"
    }
  ],
  "totalCost": 56.25
}
```

`createdAt` is omitted when unavailable. With `--show-owner`, each volume also
carries `createdBy` when it can be resolved.

## License

[MIT](LICENSE)
