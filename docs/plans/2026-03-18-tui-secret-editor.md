# TUI Secret Editor for envctl

## Summary

Add an interactive TUI editor (`envctl edit`) that enables full CRUD operations on secrets in both 1Password and AWS Secrets Manager. The editor provides a browse-and-select flow (vault picker -> item list -> field editor) built with Bubble Tea and Lip Gloss.

## Motivation

The `op-field-editor.sh` script demonstrated the value of editing 1Password field types (concealed <-> text) but the workflow of exporting to YAML, editing externally, and re-importing is clunky. A TUI editor integrated into envctl would provide a unified, interactive experience for managing secrets across backends.

## Goals

- Edit existing secret field values
- Rename secret fields/keys
- Delete secret fields/keys
- Create new secret items
- Toggle field type between concealed and text (1Password-specific)
- Browse vaults and items interactively
- Support both 1Password and AWS Secrets Manager backends

## Non-Goals

- Bulk operations (batch rename/delete across multiple items)
- Secret versioning or rollback UI
- Team/access permission management
- Syncing secrets between backends

---

## Design

### Architecture: Separate Editor Interface (Approach B)

The existing `secrets.Client` interface remains read-only. A new `secrets.Editor` interface embeds `Client` and adds write operations. Backend-specific capabilities (like 1Password field types) use optional interfaces with type assertions.

This follows Go's interface segregation idiom (like `io.Reader` / `io.ReadWriter`) — existing read-only consumers are unaffected.

### Interface and Data Model

```go
// secrets/editor.go

type Editor interface {
    Client // embeds GetSecret, GetSecretKey, Name
    ListVaults(ctx context.Context) ([]Vault, error)
    ListItems(ctx context.Context, vault string) ([]Item, error)
    GetFields(ctx context.Context, ref string) ([]Field, error)
    UpdateField(ctx context.Context, ref string, field Field) error
    DeleteField(ctx context.Context, ref string, key string) error
    RenameField(ctx context.Context, ref string, oldKey string, newKey string) error
    CreateItem(ctx context.Context, vault string, name string, fields []Field) error
}

// Optional: only 1Password implements this
type FieldTypeEditor interface {
    SetFieldType(ctx context.Context, ref string, key string, ft FieldType) error
}
```

```go
// secrets/types.go

type Vault struct {
    ID   string
    Name string
}

type Item struct {
    ID    string
    Name  string
    Vault string
}

type Field struct {
    ID      string    // backend-specific field ID
    Key     string
    Value   string
    Type    FieldType
    Section string    // 1Password sections (empty for AWS)
}

type FieldType string

const (
    FieldText      FieldType = "text"
    FieldConcealed FieldType = "concealed"
)
```

### Backend Implementations

#### 1Password (`internal/onepassword/editor.go`)

Wraps the `op` CLI, same pattern as the existing client:

| Editor Method  | `op` Command                                                 |
| -------------- | ------------------------------------------------------------ |
| ListVaults     | `op vault list --format json`                                |
| ListItems      | `op item list --vault V --format json`                       |
| GetFields      | `op item get ID --vault V --format json` -> parse `.fields[]` |
| UpdateField    | `op item edit ID 'label=newvalue'`                           |
| DeleteField    | `op item edit ID 'label[delete]'`                            |
| RenameField    | Delete old field + create new field (op has no atomic rename) |
| CreateItem     | `op item create --vault V --title N --category=SecureNote`   |
| SetFieldType   | `op item edit ID 'label[password]=val'` or `'label[text]=val'` |

Implements: `Editor` + `FieldTypeEditor`

#### AWS Secrets Manager (`internal/aws/editor.go`)

Uses the AWS SDK v2. AWS secrets are flat JSON blobs, so field operations are JSON key mutations:

| Editor Method | AWS SDK Call                                  |
| ------------- | --------------------------------------------- |
| ListVaults    | `ListSecrets` (treat path prefixes as vaults) |
| ListItems     | `ListSecrets` with prefix filter              |
| GetFields     | `GetSecretValue` -> parse JSON keys           |
| UpdateField   | Get JSON, modify key, `PutSecretValue`        |
| DeleteField   | Get JSON, remove key, `PutSecretValue`        |
| RenameField   | Get JSON, rename key, `PutSecretValue`        |
| CreateItem    | `CreateSecret` with initial JSON body         |

Implements: `Editor` only (no `FieldTypeEditor`)

Note: All AWS field mutations are atomic read-modify-write on the JSON blob.

### TUI Design

Built with Bubble Tea (Elm architecture), Lip Gloss (styling), and Bubbles (table, text input components).

#### Screen Flow

```
envctl edit
    |
    v
[Vault Picker] --enter--> [Item List] --enter--> [Field Editor]
    |                          |                       |
    q: quit               esc: back               esc: back
                          n: new item             e: edit value
                                                  d: delete field
                                                  r: rename field
                                                  t: toggle type (1P only)
                                                  n: new field
                                                  /: filter
```

#### Screen 1: Vault Picker

```
┌─ envctl edit ─────────────────────────────┐
│ Select a vault:                           │
│                                           │
│   > BACstack                              │
│     Personal                              │
│     Shared-Infra                          │
│                                           │
│ Backend: 1password  ↑/↓ navigate          │
│ enter select  q quit  / filter            │
└───────────────────────────────────────────┘
```

#### Screen 2: Item List

```
┌─ BACstack ────────────────────────────────┐
│ Select an item:                           │
│                                           │
│   > BACstack Local - Core API             │
│     BACstack Staging - Core API           │
│     BACstack Prod - Core API              │
│                                           │
│ esc back  enter select  n new item        │
│ / filter  q quit                          │
└───────────────────────────────────────────┘
```

#### Screen 3: Field Editor

```
┌─ BACstack Local - Core API ───────────────┐
│ KEY              VALUE        TYPE         │
│───────────────────────────────────────────-│
│ APP_SERVER_PORT  8080         text         │
│>DATABASE_URL     ********     concealed    │
│ REDIS_HOST       localhost    text         │
│ API_KEY          ********     concealed    │
│                                           │
│ e edit  d delete  r rename  t toggle type │
│ n new field  s save  esc back  q quit     │
│                                           │
│ [status: 2 unsaved changes]              │
└───────────────────────────────────────────┘
```

Destructive actions show a confirmation bar:
```
│ Delete DATABASE_URL? (y/n)                │
```

Inline edit mode:
```
│>DATABASE_URL     [localhost:5432___]       │
│                  esc cancel  enter save    │
```

### CLI Command

```
envctl edit                              # full browse mode
envctl edit --backend onepassword        # skip backend detection
envctl edit --vault BACstack             # skip vault picker
envctl edit --vault BACstack --item "Core API"  # skip to field editor
```

### Package Structure

```
internal/
  secrets/
    editor.go            # Editor interface + FieldTypeEditor
    types.go             # Vault, Item, Field, FieldType types
  onepassword/
    editor.go            # 1Password Editor + FieldTypeEditor impl
  aws/
    editor.go            # AWS Editor implementation
  tui/
    tui.go               # Root Bubble Tea model, screen routing
    vault_picker.go      # Screen 1: vault selection list
    item_list.go         # Screen 2: item list + create
    field_editor.go      # Screen 3: field table + CRUD ops
    confirm.go           # Confirmation overlay component
    styles.go            # Lip Gloss style definitions
    keys.go              # Key binding definitions
  cmd/
    edit.go              # `envctl edit` cobra command
```

### New Dependencies

```
github.com/charmbracelet/bubbletea   # TUI framework
github.com/charmbracelet/lipgloss    # Terminal styling
github.com/charmbracelet/bubbles     # Reusable components (table, textinput, list)
```

---

## Shared Contracts (Foundation Layer)

These types and interfaces are referenced by multiple implementation tasks and must be implemented first:

- **Interface**: `secrets.Editor` in `internal/secrets/editor.go`
- **Interface**: `secrets.FieldTypeEditor` in `internal/secrets/editor.go`
- **Types**: `Vault`, `Item`, `Field`, `FieldType` in `internal/secrets/types.go`
- **Factory**: Editor factory function in `internal/secrets/factory.go` (or extend existing)

---

## Effort Estimate

| Component                    | Estimate    |
| ---------------------------- | ----------- |
| Interfaces + types           | 0.5 days    |
| 1Password Editor impl        | 1-2 days    |
| AWS Editor impl              | 1-2 days    |
| TUI: vault picker + item list | 1-2 days   |
| TUI: field editor + CRUD     | 2-3 days    |
| CLI command + wiring         | 0.5 days    |
| Testing                      | 2-3 days    |
| **Total**                    | **~2 weeks** |

## Risks

- **`op` CLI rename**: 1Password has no atomic field rename — must delete + recreate, which could briefly lose data if interrupted
- **AWS race conditions**: Read-modify-write on JSON blobs is not atomic if multiple writers exist
- **Concealed value display**: Need to handle masking carefully in the TUI — never show concealed values by default, require explicit reveal
- **Auth scoping**: The editor inherits whatever auth the user has — no additional permission model beyond what 1P/AWS enforce
