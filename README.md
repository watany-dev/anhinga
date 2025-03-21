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
```

### Flags

- `-r, --region` (required): AWS region to query (e.g., us-east-1, us-west-2)
- `-f, --format`: Output format, either 'table', 'csv', or 'json' (default is 'table')

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

## License

[MIT](LICENSE)
