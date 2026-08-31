# tfchange

`tfchange` is a command-line tool and interactive TUI written in Go that parses Terraform JSON execution plans and renders untruncated, human‑readable resource diffs.

Unlike standard `terraform plan` or `terraform show` outputs, which omit unchanged attributes (e.g. `# (6 unchanged attributes hidden)`) or truncate long strings/blobs, `tfchange` gives you full visibility over every single resource attribute.

---
## Features
- **Interactive TUI Mode**: Navigate changed resources using `↑`/`↓` and toggle full resource diffs with `Space`.
- **Multiple Output Modes**: Supports interactive TUI (`tui`), standard text (`text`), summary table (`table`), Markdown table (`md`), and raw JSON (`json`).
- **Zero Truncation**: Prints large multi‑line strings, nested maps, and full object states intact.
- **Summary Counters**: Generates explicit change counters (e.g. `Plan: 2 to be created, 1 to be updated, 1 to be destroyed.`).
- **Color Control**: Full ANSI color support with a `-no-color` flag to strip color codes across all output modes.
- **Dynamic Value Detection**: Explicitly identifies post‑deployment dynamic values with `(known after apply)` markers.
- **Version & Verbose Flags**: `-v`/`--version` prints the binary version. `--verbose` enables verbose logging.
- **JSON Output**: `-mode=json` writes the filtered plan as a pretty‑printed JSON file suitable for scripting.

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

# Option 1: Build directly with Go
go build -o tfchange main.go

# Option 2: Use the Makefile (recommended for consistency)
make build

# (Optional) Move to system PATH
sudo mv tfchange /usr/local/bin/
```

#### Makefile Targets
- `make build` – Compile the binary.
- `make test` – Run unit tests.
- `make lint` – Run `golangci-lint` (requires installation).
- `make fmt` – Run `gofmt` and `go vet`.
- `make check` – Run `fmt`, `lint`, and `test` sequentially.
- `make clean` – Remove the built binary.
- `make help` – Show available targets.

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
- `-mode`: Set display mode (tui, text, table, md, json). Default is `tui`.
- `-no-color`: Disable colorized output across all modes.
- `-v` / `--version`: Print binary version and exit.
- `--verbose`: Enable verbose logging (useful for debugging).\n
#### Example Commands
1. **Interactive TUI Mode** (`-mode=tui`)
```bash
./tfchange -mode=tui tfplan.json
```

2. **Table Summary Mode** (`-mode=table`)
```bash
./tfchange -mode=table tfplan.json
```

3. **Markdown Mode** (`-mode=md`)
```bash
./tfchange -mode=md tfplan.json
```

4. **Text Diff Mode** (`-mode=text`)
```bash
./tfchange -mode=text tfplan.json
```

5. **JSON Output Mode** (`-mode=json`)
```bash
./tfchange -mode=json tfplan.json > filtered_plan.json
```

6. **Run all quality checks with Makefile**
```bash
make check
```

---
## Examples
### 1. Interactive TUI Mode (`-mode=tui`)
```bash
Plan: 1 to be created, 1 to be updated.
Terraform Plan Changes (Use ↑/↓ to navigate, Space to view full change, 'q' to quit):

  [+ CREATE] aws_instance.web
  [~ UPDATE] aws_security_group.app_sg
```

### 2. Table Summary Mode (`-mode=table`)
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
```bash
| Change | Resource Type | Resource Name | Address |
| --- | --- | --- | --- |
| UPDATE | aws_security_group | app_sg | aws_security_group.app_sg |
| CREATE | aws_instance | web | aws_instance.web |

Plan: 1 to be created, 1 to be updated.
```

### 4. Text Diff Mode (`-mode=text`)
```bash
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
