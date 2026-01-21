# DICOM Anonymizer - Quick Start Guide

## Step 1: Install dcmtk (Required)

dcmtk is required to process JPEG-LS compressed DICOM files.

**macOS:**
```bash
brew install dcmtk
```

**Linux (Ubuntu/Debian):**
```bash
sudo apt-get update && sudo apt-get install -y dcmtk
```

**Windows:**
Download from [dcmtk.org](https://dicom.offis.de/dcmtk.php.en) and add to PATH.

---

## Step 2: Install the Application

### macOS
1. Extract `dicom-anonymizer-macos-arm64.zip` (Apple Silicon) or `dicom-anonymizer-macos-amd64.zip` (Intel)
2. Move `DICOM Anonymizer.app` to Applications
3. First launch: Right-click → Open (to bypass Gatekeeper)

### Windows
1. Extract `dicom-anonymizer-windows-amd64.zip`
2. Run `DICOM Anonymizer.exe`

### Linux
1. Extract: `tar -xf dicom-anonymizer-linux-amd64.tar.xz`
2. Run: `./anonymizer`

---

## Step 3: Run Anonymization

### Option A: GUI Mode (Recommended for first-time users)

1. Double-click to launch the app
2. **Step 1**: Select your DICOM folder and generate a secret key
3. **Step 2**: Configure settings (defaults work for most cases)
4. **Step 3**: Preview the patient mappings
5. **Step 4**: Process files

### Option B: CLI Mode (For automation/scripting)

```bash
# Step 1: Dry run first - note the generated secret key!
./dicom-anonymizer -i /path/to/dicoms -n
# SAVE THE KEY that is displayed!

# Step 2: Process with YOUR secret key
./dicom-anonymizer -i /path/to/dicoms -k YOUR_SECRET_KEY

# Process other modalities with the SAME key
./dicom-anonymizer -i /path/to/mri_scans -k YOUR_SECRET_KEY
./dicom-anonymizer -i /path/to/ultrasound -k YOUR_SECRET_KEY

# Show all options
./dicom-anonymizer -h
```

---

## CRITICAL: Save Your Secret Key!

The secret key (`-k`) is **essential** for consistent patient anonymization.

### Why is the key important?

| Without same key | With same key |
|------------------|---------------|
| CT scan: John Smith → ANON-000001 | CT scan: John Smith → ANON-000001 |
| MRI scan: John Smith → ANON-000047 | MRI scan: John Smith → ANON-000001 |
| ❌ Cannot link patient across modalities | ✅ Same patient ID across all studies |

### What to save (and NEVER share):

1. **Secret Key**: The 32-character key (e.g., `a1b2c3d4e5f6g7h8...`)
2. **Mapping File**: `patient_mapping.json` (created automatically)

⚠️ **SECURITY WARNING**:
- The mapping file contains links between original patient IDs and anonymous IDs
- Anyone with access to this file can **re-identify patients**
- Store securely, never share, never include with anonymized data
- Only share the anonymized DICOM files in the `anonymized/` folder

### Best practice:

```bash
# Create a secure location for your keys
mkdir -p /secure/dicom-keys/

# Save your key to a file
echo "a1b2c3d4e5f6g7h8i9j0k1l2m3n4o5p6" > /secure/dicom-keys/project_key.txt

# Use the key for all modalities
./dicom-anonymizer -i /data/CT -k $(cat /secure/dicom-keys/project_key.txt)
./dicom-anonymizer -i /data/MRI -k $(cat /secure/dicom-keys/project_key.txt)
```

**Store your secret key securely and do not share it.**

---

## Output

Anonymized files are saved to:
```
{input-folder}/anonymized/ANON-XXXXXX/
```

Patient ID mappings are saved to:
```
{parent-folder}/patient_mapping.json
```

---

## Troubleshooting

### "dcmtk not installed" error
Install dcmtk using the commands in Step 1 above.

### macOS: "App is damaged" or "unidentified developer"
Right-click the app → Open → Click "Open" in the dialog.

### Files not processing
- Check that files are valid DICOM format
- Ensure input folder path is correct
- Try running with `--retry` flag to retry failed files

---

## Need Help?

See the full documentation in `README.md` or contact support.
