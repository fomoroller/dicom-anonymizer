# DICOM Anonymizer

A cross-platform GUI application for anonymizing DICOM medical imaging files. Built with Go and Fyne.

## Features

- **Metadata Anonymization**: Removes or modifies patient identifying information from DICOM tags
- **Pixel Redaction**: Blacks out burned-in PHI from ultrasound images (configurable rows)
- **Patient Pseudonymization**: Generates consistent anonymous IDs using a secret key
- **Multi-modality Support**: CT, MRI, X-Ray, and Ultrasound
- **Progress Tracking**: Resume interrupted anonymization runs
- **Cross-platform**: macOS (ARM64 & Intel), Windows, and Linux

## Downloads

Download the latest release for your platform:

| Platform | Architecture | Download |
|----------|--------------|----------|
| macOS | Apple Silicon (M1/M2/M3) | [dicom-anonymizer-macos-arm64.zip](build/releases/dicom-anonymizer-macos-arm64.zip) |
| macOS | Intel | [dicom-anonymizer-macos-amd64.zip](build/releases/dicom-anonymizer-macos-amd64.zip) |
| Windows | 64-bit | [dicom-anonymizer-windows-amd64.zip](build/releases/dicom-anonymizer-windows-amd64.zip) |
| Linux | 64-bit | [dicom-anonymizer-linux-amd64.tar.xz](build/releases/dicom-anonymizer-linux-amd64.tar.xz) |

## Requirements

### dcmtk (Required)

This application requires **dcmtk** to process JPEG-LS compressed DICOM files. The app will prompt you to install it on first run.

**macOS (Homebrew):**
```bash
brew install dcmtk
```

**Linux (Debian/Ubuntu):**
```bash
sudo apt-get update && sudo apt-get install -y dcmtk
```

**Windows:**
Download from [dcmtk.org](https://dicom.offis.de/dcmtk.php.en) and add to PATH.

## Installation

### macOS

1. Download `dicom-anonymizer-macos-arm64.zip` (Apple Silicon) or `dicom-anonymizer-macos-amd64.zip` (Intel)
2. Extract the zip file
3. Move `dicom-anonymizer.app` to your Applications folder
4. On first launch, right-click and select "Open" to bypass Gatekeeper

### Windows

1. Download `dicom-anonymizer-windows-amd64.zip`
2. Extract the zip file
3. Run `dicom-anonymizer.exe`

### Linux

1. Download `dicom-anonymizer-linux-amd64.tar.xz`
2. Extract: `tar -xf dicom-anonymizer-linux-amd64.tar.xz`
3. Run: `./dicom-anonymizer`

## Usage

### GUI Mode (Default)

Simply run the application without arguments to launch the graphical interface.

```bash
./dicom-anonymizer
```

### CLI Mode

Run from the command line for automation and scripting.

#### Important: Secret Key & Mapping File Security

The secret key (`-k`) and mapping file are **critical** for consistent patient anonymization:

- **You MUST use the same key** when processing different modalities (CT, MRI, Ultrasound, X-Ray) for the same patients
- The key ensures patient "John Smith" receives the **same anonymous ID** (e.g., `ANON-000001`) across ALL imaging studies
- If you lose the key, you **cannot maintain patient ID consistency**
- **Save your key securely** - store it alongside your mapping file

⚠️ **SECURITY WARNING - DO NOT SHARE:**
| File | Risk if shared |
|------|----------------|
| Secret Key | Allows re-generation of patient ID mappings |
| `patient_mapping.json` | Contains direct links between real and anonymous IDs - **enables patient re-identification** |

**Only share the anonymized DICOM files** in the `anonymized/` output folder.

#### Workflow: Processing Multiple Modalities

```bash
# Step 1: Dry run to generate and preview (note the generated key)
./dicom-anonymizer -i /data/CT_Scans -n
# Output shows: Key: a1b2c3d4e5f6g7h8...
# SAVE THIS KEY!

# Step 2: Process CT scans with your key
./dicom-anonymizer -i /data/CT_Scans -k a1b2c3d4e5f6g7h8

# Step 3: Process MRI scans with the SAME key
./dicom-anonymizer -i /data/MRI_Scans -k a1b2c3d4e5f6g7h8

# Step 4: Process Ultrasound with the SAME key
./dicom-anonymizer -i /data/Ultrasound -k a1b2c3d4e5f6g7h8

# Result: Patient "John Smith" has the same ANON-XXXXXX ID across all modalities
```

#### CLI Flags

| Flag | Short | Default | Description |
|------|-------|---------|-------------|
| `--input` | `-i` | (required) | Input folder containing DICOM files |
| `--key` | `-k` | auto-generate | **Secret key for pseudonymization (SAVE THIS!)** |
| `--mapping` | `-m` | `{parent}/patient_mapping.json` | Patient mapping file path |
| `--redact-rows` | | 75 | Rows to redact from ultrasound images |
| `--recursive` | `-r` | true | Search subdirectories |
| `--retry` | | false | Retry previously failed files |
| `--metadata` | | true | Process CT/MRI/X-Ray (metadata only) |
| `--ultrasound` | | true | Process ultrasound (metadata + pixel redaction) |
| `--dry-run` | `-n` | false | Preview only, no files modified |
| `--help` | `-h` | | Show help message |

#### CLI Examples

```bash
# Always do a dry run first
./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY -n

# Process all files
./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY

# Process only CT/MRI/X-Ray (skip ultrasound pixel redaction)
./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY --ultrasound=false

# Process only ultrasound with custom redaction height
./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY --metadata=false --redact-rows=100

# Retry failed files from previous run
./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY --retry

# Use custom mapping file location
./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY -m /secure/patient_mappings.json

# Show full help
./dicom-anonymizer -h
```

#### CLI Output

```
DICOM Anonymizer
==================================================
Input:     /path/to/dicoms
Mapping:   /path/to/patient_mapping.json
Key:       a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

WARNING: Secret key was auto-generated!
         SAVE THIS KEY to maintain consistent patient IDs
         across different imaging modalities (CT, MRI, US, X-Ray).
         Re-run with: -k a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6

Modality:  CT/MRI/X-Ray, Ultrasound (75px redaction)
Options:   Recursive

[##################################################] 100%  (156/156)

==================================================
Complete! 150 succeeded, 4 failed, 2 skipped
Patients:  12 total (10 by Name+DOB, 2 by PatientID)
Output:    /path/to/dicoms/anonymized
Mapping:   /path/to/patient_mapping.json
```

---

### GUI Wizard

#### Step 1: Select Input

1. Click **Browse** to select the folder containing your DICOM files
2. Enter or generate a **Secret Key** (required for consistent anonymization)
   - Click **Generate** to create a new key
   - **Important**: Save this key securely! You'll need it to maintain consistent patient IDs

### Step 2: Configure Settings

- **Modality Types**: Select which types of files to process
  - CT / MRI / X-Ray: Metadata anonymization only
  - Ultrasound: Metadata + pixel redaction (burns out top N rows)
- **Options**:
  - Search subdirectories: Process files in subfolders
  - Retry failed files: Re-attempt previously failed files
- **Mapping File**: Location to store patient ID mappings

### Step 3: Preview

Review the files that will be processed and the patient ID mappings.

### Step 4: Process

Click **Process** to begin anonymization. Progress is shown in real-time.

## Anonymization Details

### Fields Cleared
- Patient Name, Birth Date, Age, Address, Phone
- All physician names (referring, performing, operators)
- Accession Number, Study ID
- Institution Address, Department Name, Station Name
- Times (Study, Series, Acquisition, Content)

### Fields Preserved
- Patient Sex (clinical relevance)
- Institution Name (research tracking)
- Study/Series Description (clinical context)

### Date Handling
- Dates are truncated to the 1st of the month (e.g., 20260115 -> 20260101)

### Ultrasound Pixel Redaction
- Top N rows are blacked out to remove burned-in PHI
- Default: 75 pixels from top

## Building from Source

### Prerequisites

- Go 1.21 or later
- Fyne dependencies (see [fyne.io](https://fyne.io/develop/))

**macOS:**
```bash
xcode-select --install
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt install libgl1-mesa-dev xorg-dev
```

### Build

```bash
# Install dependencies
make deps

# Build native binary
make build

# Run
make run
```

### Cross-compile

Requires Docker and fyne-cross:

```bash
# Install fyne-cross
go install github.com/fyne-io/fyne-cross@v1.5.0

# Build for all platforms
make cross-compile
```

## Project Structure

```
dicom-anonymizer/
├── cmd/anonymizer/      # Main application entry point
├── internal/
│   ├── anonymizer/      # Core anonymization logic
│   ├── cli/             # Command-line interface
│   ├── dicom/           # DICOM file reading/writing
│   ├── gui/             # Fyne GUI (wizard, theme)
│   ├── identity/        # Patient pseudonymization
│   ├── jpegls/          # JPEG-LS encoder/decoder
│   └── progress/        # Progress tracking
├── build/
│   └── releases/        # Pre-built binaries
└── Makefile
```

## License

MIT License
