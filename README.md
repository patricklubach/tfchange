# tfchange

`tfchange` is a command-line tool and interactive TUI written in Go that parses Terraform JSON execution plans and renders untruncated, human-readable resource diffs.

Unlike standard `terraform plan` or `terraform show` outputs, which omit unchanged attributes (e.g. `# (6 unchanged attributes hidden)`) or truncate long strings/blobs, `tfchange` gives you full visibility over every single resource attribute.

---

## Features

- **Interactive TUI Mode**: Navigate changed resources using `↑`/`↓` and toggle full resource diffs with `Space`.
- **Multiple Output Modes**: Supports interactive TUI (`tui`), standard text (`text`), summary table (`table`), and Markdown table (`md`).
- **Zero Truncation**: Prints large multi-line strings, nested maps, and full object states intact.
- **Summary Counters**: Generates explicit change counters (e.g. `Plan: 2 to be created, 1 to be updated, 1 to be destroyed.`).
- **Color Control**: Full ANSI color support with a `-no-color` flag to strip color codes across all output modes.
- **Dynamic Value Detection**: Explicitly identifies post-deployment dynamic values with `(known after apply)` markers.
---

## Installation

### Prerequisites

- **Go**: Version 1.20 or higher.
- **Terraform**: Required to generate binary execution plans.

### Build from Source

```bash
# Clone the repository
git clone https://github.com/patricklubach/tfchange.git
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

### 2. Output Modes & Flags

#### Flags

* `-mode`: Set display mode (tui, text, table, md). Default is tui.
* `-no-color`: Disable colorized output across all modes.

## Examples

### 1. Interactive TUI Mode (`-mode=tui`)

Default view when running `tfchange tfplan.json`:

```bash
Plan: 1 to be created, 1 to be updated.
Terraform Plan Changes (Use ↑/↓ to navigate, Space to view full change, 'q' to quit):

> [+ CREATE] aws_instance.web
  [~ UPDATE] aws_security_group.app_sg
```

> Press Space on any selected item to view the untruncated diff inside a full viewport:

```bash
Resource Change Details (Press Space or 'q' to close):

# aws_security_group.app_sg will be updated in-place
  ~ resource "aws_security_group" "app_sg" {
      ~ description = "Old SG" -> "New SG"
      + id = (known after apply)
    }
```

### 2. Table Summary Mode (`-mode=table`)

Provides a tf-summarize-like tabular overview:

```bash
$ tfchange -mode=table tfplan.json

+--------+--------------------+---------------+---------------------------+
| CHANGE |   RESOURCE TYPE    | RESOURCE NAME |          ADDRESS          |
+--------+--------------------+---------------+---------------------------+
| UPDATE | aws_security_group | app_sg        | aws_security_group.app_sg |
| CREATE | aws_instance       | web           | aws_instance.web          |
+--------+--------------------+---------------+---------------------------+
Plan: 1 to be created, 1 to be updated.
```

### 3. Markdown Mode (`-mode=md`)

Generates valid Markdown tables suitable for GitHub PR comments or documentation:

```bash
tfchange -mode=md tfplan.json

| Change | Resource Type | Resource Name | Address |
| --- | --- | --- | --- |
| UPDATE | aws_security_group | app_sg | aws_security_group.app_sg |
| CREATE | aws_instance | web | aws_instance.web |

Plan: 1 to be created, 1 to be updated.
```

### 4. Text Diff Mode (`-mode=text`)

Outputs raw, untruncated diffs directly to stdout:

```bash
tfchange -mode=text tfplan.json

# aws_security_group.app_sg will be updated in-place
  ~ resource "aws_security_group" "app_sg" {
      ~ description = "Old SG" -> "New SG"
      + id = (known after apply)
    }

# aws_instance.web will be created
  + resource "aws_instance" "web" {
      + instance_type = "t3.micro"
      + id = (known after apply)
    }

Plan: 1 to be created, 1 to be updated.
```

---

## License

[MIT](LICENSE)
