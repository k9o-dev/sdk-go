package k9o

import (
	"context"
	"encoding/json"
	"net/url"
)

// ColumnReferences is a foreign-key reference on a column — compiles to a
// REFERENCES clause. OnDelete/OnUpdate accept "CASCADE" | "RESTRICT" |
// "SET NULL" | "NO ACTION".
type ColumnReferences struct {
	Collection string `json:"collection"`
	On         string `json:"on"`
	OnDelete   string `json:"on_delete,omitempty"`
	OnUpdate   string `json:"on_update,omitempty"`
}

// ColumnEncryption is the object form of a column's at-rest field encryption
//. The wire form is the union `true | { store, searchable? }`:
// the zero value (Store == "") marshals to the bare `true` shorthand (inline-
// stored encrypted column); setting Store opts into "inline" | "vault" and the
// D-mode searchable-encryption index. NOTE: Searchable is NESTED here, not a
// sibling of the column.
type ColumnEncryption struct {
	Store      string `json:"store,omitempty"`
	Searchable bool   `json:"searchable,omitempty"`
}

// MarshalJSON renders the `true | { store, searchable? }` union: the inline
// shorthand collapses to `true`; otherwise the object form (store required).
func (e ColumnEncryption) MarshalJSON() ([]byte, error) {
	if e.Store == "" {
		return []byte("true"), nil
	}
	type wire struct {
		Store      string `json:"store"`
		Searchable bool   `json:"searchable,omitempty"`
	}
	return json.Marshal(wire{Store: e.Store, Searchable: e.Searchable})
}

// ColumnSpec is a column definition. Type is the column-type
// vocabulary (parser-owned; kept as a string rather than a fabricated enum).
type ColumnSpec struct {
	Name         string            `json:"name"`
	Type         string            `json:"type"`
	PrimaryKey   bool              `json:"primary_key,omitempty"`
	NotNull      bool              `json:"not_null,omitempty"`
	Unique       bool              `json:"unique,omitempty"`
	Default      string            `json:"default,omitempty"`
	References   *ColumnReferences `json:"references,omitempty"`
	Encrypted    *ColumnEncryption `json:"encrypted,omitempty"`
	BlindIndexed bool              `json:"blind_indexed,omitempty"`
}

// IndexSpec is an index definition. Name defaults to <table>_<cols>_idx.
type IndexSpec struct {
	Name    string   `json:"name,omitempty"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique,omitempty"`
}

// CreateTableOp is the payload of an Operation.CreateTable.
type CreateTableOp struct {
	Name    string       `json:"name"`
	Columns []ColumnSpec `json:"columns"`
	Indexes []IndexSpec  `json:"indexes,omitempty"`
}

// AddColumnOp is the payload of an Operation.AddColumn.
type AddColumnOp struct {
	Table  string     `json:"table"`
	Column ColumnSpec `json:"column"`
}

// DropColumnOp is the payload of an Operation.DropColumn.
type DropColumnOp struct {
	Table  string `json:"table"`
	Column string `json:"column"`
}

// AlterTableOp is the payload of an Operation.AlterTable — the BATCHED column
// mutation: one op carries optional AddColumns + DropColumns (NOT separate
// add/drop ops). AddColumns entries are full ColumnSpec (same shape as
// create_table columns, incl the encrypted / blind_indexed field-protection
// flags); DropColumns is column names. Live in the gateway runner
// (migration-runner.ts:156).
type AlterTableOp struct {
	Name        string       `json:"name"`
	AddColumns  []ColumnSpec `json:"add_columns,omitempty"`
	DropColumns []string     `json:"drop_columns,omitempty"`
}

// CreateIndexOp is the payload of an Operation.CreateIndex.
type CreateIndexOp struct {
	Table string    `json:"table"`
	Index IndexSpec `json:"index"`
}

// DropIndexOp is the payload of an Operation.DropIndex.
type DropIndexOp struct {
	Name string `json:"name"`
}

// Operation is a single declarative DDL operation — KEY-TAGGED (exactly one
// field set), the shape the gateway applies. The v2 set is these 7 — no
// alter_column, rename_table, or constraint ops at v2. alter_table is the
// BATCHED column mutation (live in the gateway runner).
// rename_column remains a pending future op (its parser field names are not yet
// defined) and is added in one bump when it lands. Go has no sum types, so the
// union is modelled as a struct of omitempty pointers; set exactly one.
type Operation struct {
	CreateTable *CreateTableOp `json:"create_table,omitempty"`
	DropTable   *string        `json:"drop_table,omitempty"`
	AddColumn   *AddColumnOp   `json:"add_column,omitempty"`
	DropColumn  *DropColumnOp  `json:"drop_column,omitempty"`
	AlterTable  *AlterTableOp  `json:"alter_table,omitempty"`
	CreateIndex *CreateIndexOp `json:"create_index,omitempty"`
	DropIndex   *DropIndexOp   `json:"drop_index,omitempty"`
}

// ActionRule is a declarative authorization/relationship rule (compiles to RLS
// USING + WITH CHECK). The rule grammar is the policy-expression
// DSL; Rule is left as any to mirror the spec's open shape.
type ActionRule struct {
	Rule any `json:"rule,omitempty"`
}

// RelationshipSpec is a declared relationship for embedding/joins.
// Type accepts "inner" | "left".
type RelationshipSpec struct {
	Collection string `json:"collection"`
	On         string `json:"on"`
	As         string `json:"as"`
	Type       string `json:"type,omitempty"`
}

// CollectionActions are the per-action RLS rules on a collection.
type CollectionActions struct {
	Select *ActionRule `json:"select,omitempty"`
	Insert *ActionRule `json:"insert,omitempty"`
	Update *ActionRule `json:"update,omitempty"`
	Delete *ActionRule `json:"delete,omitempty"`
}

// CollectionPolicy is a per-collection policy block (RLS + relationships),.
type CollectionPolicy struct {
	References []RelationshipSpec `json:"references,omitempty"`
	Actions    *CollectionActions `json:"actions,omitempty"`
}

// StorageBucketActions are the per-action rules on a storage bucket.
type StorageBucketActions struct {
	Read   *ActionRule `json:"read,omitempty"`
	Upload *ActionRule `json:"upload,omitempty"`
	Delete *ActionRule `json:"delete,omitempty"`
	List   *ActionRule `json:"list,omitempty"`
}

// StorageBucketPolicy is a per-bucket storage policy block,.
type StorageBucketPolicy struct {
	Public  bool                  `json:"public,omitempty"`
	Actions *StorageBucketActions `json:"actions,omitempty"`
}

// MigrationStorage is the storage section of a MigrationPolicy.
type MigrationStorage struct {
	Buckets map[string]StorageBucketPolicy `json:"buckets,omitempty"`
}

// MigrationPolicy is the optional policy section authored alongside a migration.
type MigrationPolicy struct {
	Collections map[string]CollectionPolicy `json:"collections,omitempty"`
	Storage     *MigrationStorage           `json:"storage,omitempty"`
}

// Migration is a versioned, checksummed unit of structured DDL.
// Up applies the change; Down reverses it. Both are operation arrays.
type Migration struct {
	Version int              `json:"version"`
	Slug    string           `json:"slug"`
	Up      []Operation      `json:"up"`
	Down    []Operation      `json:"down"`
	Policy  *MigrationPolicy `json:"policy,omitempty"`
}

// AppliedMigration is returned by Migrations.List + Get.
type AppliedMigration struct {
	Version   int    `json:"version"`
	Slug      string `json:"slug"`
	Checksum  string `json:"checksum"`
	AppliedAt string `json:"appliedAt"`
}

// PendingMigration is returned by Migrations.ListPending.
type PendingMigration struct {
	PendingID   string `json:"pendingId"`
	Version     int    `json:"version"`
	Slug        string `json:"slug"`
	Checksum    string `json:"checksum"`
	SubmittedAt string `json:"submittedAt"`
	SubmittedBy string `json:"submittedBy"`
}

// MigrationsNamespace exposes client.Migrations.*.
type MigrationsNamespace struct {
	config *httpClientConfig
}

// Apply applies migrations immediately (PROJECT_ADMIN minimum).
func (n *MigrationsNamespace) Apply(ctx context.Context, migrations []Migration) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "migrations")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "migrations/apply"), Body: map[string]any{"migrations": migrations}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// List returns applied migrations on the project.
func (n *MigrationsNamespace) List(ctx context.Context) ([]AppliedMigration, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "migrations")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[[]AppliedMigration](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "migrations")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// Get returns a single applied migration by version.
func (n *MigrationsNamespace) Get(ctx context.Context, version string) (*AppliedMigration, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "migrations")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[AppliedMigration](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "migrations/"+url.PathEscape(version))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// Queue queues migrations for maker-checker approval.
func (n *MigrationsNamespace) Queue(ctx context.Context, migrations []Migration) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "migrations")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "migrations/queue"), Body: map[string]any{"migrations": migrations}})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// ListPending returns pending (queued, not yet approved) migrations.
func (n *MigrationsNamespace) ListPending(ctx context.Context) ([]PendingMigration, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "migrations")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[[]PendingMigration](ctx, n.config, &requestInit{Method: "GET", Path: ProjectPath(slug, "migrations/pending")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// ApplyPending approves + applies a pending migration.
func (n *MigrationsNamespace) ApplyPending(ctx context.Context, pendingID string) (*AppliedMigration, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "migrations")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[AppliedMigration](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "migrations/pending/"+url.PathEscape(pendingID)+"/apply")})
	if kerr != nil {
		return nil, nil, kerr
	}
	return &env.Data, &env.Meta, nil
}

// RejectPending rejects a pending migration (does not apply).
func (n *MigrationsNamespace) RejectPending(ctx context.Context, pendingID string, reason string) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "migrations")
	if kerr != nil {
		return nil, nil, kerr
	}
	var body any
	if reason != "" {
		body = map[string]any{"reason": reason}
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "POST", Path: ProjectPath(slug, "migrations/pending/"+url.PathEscape(pendingID)+"/reject"), Body: body})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}

// DeletePending deletes a pending migration from the queue.
func (n *MigrationsNamespace) DeletePending(ctx context.Context, pendingID string) (map[string]any, *Meta, *KError) {
	slug, kerr := requireProjectSlug(n.config, "migrations")
	if kerr != nil {
		return nil, nil, kerr
	}
	env, kerr := doRequest[map[string]any](ctx, n.config, &requestInit{Method: "DELETE", Path: ProjectPath(slug, "migrations/pending/"+url.PathEscape(pendingID))})
	if kerr != nil {
		return nil, nil, kerr
	}
	return env.Data, &env.Meta, nil
}
