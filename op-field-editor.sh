#!/usr/bin/env bash
# op-field-editor.sh — Export a 1Password item's fields to a YAML file for editing,
# then re-import with updated field types (e.g., concealed → text).
#
# Usage:
#   ./scripts/op-field-editor.sh export --vault "BACstack" --item "BACstack Local - Core API"
#   # Edit the generated YAML: change "type: CONCEALED" to "type: TEXT" for non-secret fields
#   ./scripts/op-field-editor.sh import --file fields-BACstack-Local---Core-API.yaml
#
# The YAML format per field:
#   - label: APP_SERVER_PORT
#     value: "8080"
#     type: TEXT          # TEXT or CONCEALED
#     section: ""
#
# On import, fields whose type changed are updated via `op item edit`.

set -euo pipefail

COMMAND="${1:-}"
shift || true

usage() {
    cat <<'EOF'
Usage:
  op-field-editor.sh export --vault VAULT --item ITEM [--account ACCOUNT]
  op-field-editor.sh import --file FILE [--account ACCOUNT]

Commands:
  export   Fetch item fields and write to a YAML file for editing
  import   Read edited YAML and update changed field types in 1Password

Options:
  --vault VAULT       1Password vault name
  --item ITEM         1Password item name or ID
  --file FILE         Path to the edited YAML file (for import)
  --account ACCOUNT   1Password account shorthand (default: bactrack)
EOF
    exit 1
}

VAULT=""
ITEM=""
FILE=""
ACCOUNT="bactrack"

while [[ $# -gt 0 ]]; do
    case "$1" in
        --vault)   VAULT="$2"; shift 2 ;;
        --item)    ITEM="$2"; shift 2 ;;
        --file)    FILE="$2"; shift 2 ;;
        --account) ACCOUNT="$2"; shift 2 ;;
        *)         echo "Unknown option: $1"; usage ;;
    esac
done

OP_ARGS=(--account "$ACCOUNT")

check_signed_in() {
    if ! op whoami "${OP_ARGS[@]}" &>/dev/null; then
        echo "Error: Not signed in to 1Password. Run: op signin --account $ACCOUNT"
        exit 1
    fi
}

do_export() {
    [[ -z "$VAULT" ]] && { echo "Error: --vault required"; usage; }
    [[ -z "$ITEM" ]] && { echo "Error: --item required"; usage; }

    check_signed_in

    # Sanitize item name for filename
    safe_name=$(echo "$ITEM" | tr ' ' '-' | tr -cd '[:alnum:]-_')
    outfile="fields-${safe_name}.yaml"

    echo "Fetching item '$ITEM' from vault '$VAULT'..."

    # Get item as JSON
    item_json=$(op item get "$ITEM" --vault "$VAULT" "${OP_ARGS[@]}" --format json)

    item_id=$(echo "$item_json" | python3 -c "import json,sys; print(json.load(sys.stdin)['id'])")

    # Parse fields into YAML
    python3 -c "
import json, sys

item = json.loads('''$(echo "$item_json" | python3 -c "import json,sys; print(json.dumps(json.load(sys.stdin)))")''')

print(f'# 1Password Field Editor')
print(f'# Item: {item[\"title\"]}')
print(f'# ID: {item[\"id\"]}')
print(f'# Vault: $(echo "$VAULT")')
print(f'#')
print(f'# Edit the \"type\" field to change between CONCEALED and TEXT.')
print(f'# Only type changes are applied on import — values are not modified.')
print(f'#')
print(f'# Types: TEXT, CONCEALED')
print()
print('item_id: \"' + item['id'] + '\"')
print('vault: \"$(echo "$VAULT")\"')
print('fields:')

for field in item.get('fields', []):
    label = field.get('label', '')
    if not label:
        continue
    value = field.get('value', '')
    ftype = field.get('type', 'STRING')
    section = ''
    if 'section' in field and field['section']:
        section = field['section'].get('label', field['section'].get('id', ''))
    field_id = field.get('id', '')

    # Normalize type names
    display_type = 'CONCEALED' if ftype == 'CONCEALED' else 'TEXT'

    # Mask concealed values for safety
    display_value = '********' if ftype == 'CONCEALED' else value

    print(f'  - label: \"{label}\"')
    print(f'    id: \"{field_id}\"')
    print(f'    value: \"{display_value}\"')
    print(f'    type: {display_type}')
    if section:
        print(f'    section: \"{section}\"')
    print()
" > "$outfile"

    echo ""
    echo "Exported to: $outfile"
    echo ""
    echo "Edit the file and change 'type: CONCEALED' to 'type: TEXT' for non-secret fields."
    echo "Then run: $0 import --file $outfile"
}

do_import() {
    [[ -z "$FILE" ]] && { echo "Error: --file required"; usage; }
    [[ ! -f "$FILE" ]] && { echo "Error: File not found: $FILE"; exit 1; }

    check_signed_in

    echo "Reading $FILE..."

    # Parse YAML and get current item state, then apply changes
    python3 -c "
import subprocess, sys, re

# Simple YAML parser for our known format
with open('$FILE') as f:
    content = f.read()

# Extract item_id and vault
item_id_match = re.search(r'item_id:\s*\"(.+?)\"', content)
vault_match = re.search(r'vault:\s*\"(.+?)\"', content)
if not item_id_match or not vault_match:
    print('Error: Could not find item_id or vault in file')
    sys.exit(1)

item_id = item_id_match.group(1)
vault = vault_match.group(1)

# Parse fields
fields = []
current = {}
for line in content.split('\n'):
    line = line.strip()
    if line.startswith('- label:'):
        if current:
            fields.append(current)
        current = {'label': re.search(r'\"(.+?)\"', line).group(1)}
    elif line.startswith('id:') and current:
        m = re.search(r'\"(.+?)\"', line)
        if m:
            current['id'] = m.group(1)
    elif line.startswith('type:') and current:
        current['type'] = line.split(':')[1].strip()
    elif line.startswith('section:') and current:
        m = re.search(r'\"(.+?)\"', line)
        if m:
            current['section'] = m.group(1)
if current:
    fields.append(current)

# Get current item state
import json
result = subprocess.run(
    ['op', 'item', 'get', item_id, '--vault', vault, '--account', '$ACCOUNT', '--format', 'json'],
    capture_output=True, text=True
)
if result.returncode != 0:
    print(f'Error fetching item: {result.stderr}')
    sys.exit(1)

current_item = json.loads(result.stdout)
current_fields = {f.get('label', ''): f for f in current_item.get('fields', []) if f.get('label')}

# Find type changes
changes = []
for field in fields:
    label = field['label']
    new_type = field['type']
    if label in current_fields:
        cur = current_fields[label]
        cur_type = 'CONCEALED' if cur.get('type') == 'CONCEALED' else 'TEXT'
        if cur_type != new_type:
            changes.append({
                'label': label,
                'id': field.get('id', cur.get('id', '')),
                'from': cur_type,
                'to': new_type,
                'value': cur.get('value', ''),
                'section': field.get('section', ''),
            })

if not changes:
    print('No type changes detected.')
    sys.exit(0)

print(f'Found {len(changes)} field(s) to update:')
for c in changes:
    print(f'  {c[\"label\"]}: {c[\"from\"]} -> {c[\"to\"]}')
print()

# Apply changes
for c in changes:
    label = c['label']
    field_type = 'password' if c['to'] == 'CONCEALED' else 'text'
    section = c.get('section', '')

    # Build the assignment string
    # op item edit uses: [section.]label[type]=value
    if section:
        assignment = f'{section}.{label}[{field_type}]={c[\"value\"]}'
    else:
        assignment = f'{label}[{field_type}]={c[\"value\"]}'

    cmd = ['op', 'item', 'edit', item_id, '--vault', vault, '--account', '$ACCOUNT', assignment]
    print(f'Updating {label} to {c[\"to\"]}...')
    result = subprocess.run(cmd, capture_output=True, text=True)
    if result.returncode != 0:
        print(f'  Error: {result.stderr.strip()}')
    else:
        print(f'  Done.')

print()
print('All changes applied.')
"
}

case "$COMMAND" in
    export) do_export ;;
    import) do_import ;;
    *)      usage ;;
esac
