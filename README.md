# tfchange

`tfchange` is a lightweight command-line utility written in Go that parses Terraform JSON execution plans and renders untruncated, human-readable resource diffs.

Unlike standard `terraform plan` or `terraform show` commands, which automatically truncate large strings, complex nested objects, or omit unchanged fields (e.g. `# (6 unchanged attributes hidden)`), `tfchange` prints every attribute and value completely expanded—including explicit `(known after apply)` dynamic markers.

---

## Features

- **Zero Truncation**: Full string contents, large JSON blobs, and complex objects are displayed without ellipsis (`...`).
- **No Hidden Attributes**: Shows all attributes on modified resources, ensuring complete visibility during code reviews and security auditing.
- **Terraform-Native Formatting**: Uses standard diff markers (`+`, `-`, `~`, `-/+`) matching official Terraform plan conventions.
- **Dynamic Value Detection**: Accurately parses `after_unknown` schema maps to display `(known after apply)` markers for values determined post-deployment.
- **Pipeline Ready**: Accepts piped standard input (`stdin`) or direct file paths.

---

## Installation

### Prerequisites

- **Go**: Version 1.18 or higher.
- **Terraform**: Required to generate binary execution plans.

### Build from Source

```bash
# Clone the repository
git clone https://github.com/your-username/tfchange.git
cd tfchange

# Build the binary
go build -o tfchange main.go

# (Optional) Move to system PATH
sudo mv tfchange /usr/local/bin/
```

---

## Usage

### 1. Generate Terraform Plan JSON

Export your binary Terraform plan to JSON using `terraform show -json`:

```bash
terraform plan -out=tfplan
terraform show -json tfplan > tfplan.json
```

### 2. Run `tfchange`

#### Pass file path as positional argument:
```bash
tfchange tfplan.json
```

#### Pipe directly via stdin:
```bash
terraform show -json tfplan | tfchange
```

---

## Output Example

```hcl
# aws_security_group.app_sg will be updated in-place
  ~ resource "aws_security_group" "app_sg" {
      ~ description = "Production SG" -> "Production App SG - Internal"
        id = "sg-0123456789abcdef0"
      ~ ingress = [
          {
            "cidr_blocks": [
              "10.0.0.0/16"
            ],
            "from_port": 80,
            "protocol": "tcp",
            "to_port": 80
          }
        ]
      + ingress_rule_id = (known after apply)
        name = "app-service-sg"
      ~ tags = {
          "Environment": "production",
          "ManagedBy": "Terraform"
        }
    }
```

---

## License

[MIT](LICENSE)
