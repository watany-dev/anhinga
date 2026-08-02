# Anhinga

Anhinga is a Go CLI tool that lists AWS EBS volumes and calculates their monthly costs based on volume type and size.

## Features

- List all EBS volumes in a specified AWS region
- Calculate the monthly cost for each EBS volume
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
git clone https://github.com/username/anhinga.git
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

- `-r, --region` (required): AWS region to query (e.g., us-east-1, us-west-2)
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

## Example Output

### Table Format

```
+------------------+---------+-----------+---------+-----------------+
|    VOLUME ID     |  TYPE   | SIZE (GB) |  STATE  | MONTHLY COST ($)|
+------------------+---------+-----------+---------+-----------------+
| vol-12345678     | gp2     |     100   | in-use  |           10.00 |
| vol-87654321     | io1     |      50   | in-use  |            6.25 |
| vol-11223344     | gp3     |     500   | in-use  |           40.00 |
+------------------+---------+-----------+---------+-----------------+
|                                         | TOTAL   |           56.25 |
+------------------+---------+-----------+---------+-----------------+
Total EBS Monthly Cost: $56.25
```

### Table Format with `--show-owner`

```
+--------------+------+-----------+-----------+---------------------------+------------+------------------+
|  VOLUME ID   | TYPE | SIZE (GB) |   STATE   |        CREATED BY         | CREATED AT | MONTHLY COST ($) |
+--------------+------+-----------+-----------+---------------------------+------------+------------------+
| vol-12345678 | gp2  |       100 | in-use    | DeployRole/alice          | 2026-06-14 |            10.00 |
| vol-87654321 | io1  |        50 | available | unknown (>90d)            |            |             6.25 |
| vol-11223344 | gp3  |       500 | in-use    | autoscaling.amazonaws.com | 2026-07-02 |            40.00 |
+--------------+------+-----------+-----------+---------------------------+------------+------------------+
|                                                                             TOTAL    |      56.25       |
+--------------+------+-----------+-----------+---------------------------+------------+------------------+
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
      "type": "gp2",
      "sizeGb": 100,
      "state": "in-use",
      "monthlyCost": 10.00
    },
    {
      "volumeId": "vol-87654321",
      "type": "io1",
      "sizeGb": 50,
      "state": "in-use",
      "monthlyCost": 6.25
    },
    {
      "volumeId": "vol-11223344",
      "type": "gp3",
      "sizeGb": 500,
      "state": "in-use",
      "monthlyCost": 40.00
    }
  ],
  "totalMonthlyCost": 56.25
}
```

With `--show-owner`, each volume additionally carries `createdBy` and
`createdAt` (both omitted when unavailable).

## License

[MIT](LICENSE)
