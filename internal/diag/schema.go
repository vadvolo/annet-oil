package diag

import _ "embed"

// SchemaProto is the protobuf schema (source of truth) served to clients so they
// can generate their own stubs. Keep internal/diag/diag.proto in sync with
// proto/diag/v1/diag.proto (the Makefile `proto` target regenerates both).
//
//go:embed diag.proto
var SchemaProto string
