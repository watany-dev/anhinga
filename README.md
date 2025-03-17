# Anhinga

Anhinga is a Go CLI tool that lists AWS EBS volumes and calculates their monthly costs based on volume type and size.

## Features

- List all EBS volumes in a specified AWS region
- Calculate the monthly cost for each EBS volume
- Display results in table or CSV format
- Show total cost of all EBS volumes

## Installation

```bash
go install github.com/anhinga/anhinga@latest
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
```

### Flags

- `-r, --region` (required): AWS region to query (e.g., us-east-1, us-west-2)
- `-f, --format`: Output format, either 'table' or 'csv' (default is 'table')

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

## License

[MIT](LICENSE)