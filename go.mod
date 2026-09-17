module github.com/ppat/mediated-mailbox-mcp

go 1.27.1

ignore (
	./ui/browser/codegen/node_modules
	./ui/browser/node_modules
)

require (
	github.com/getkin/kin-openapi v0.149.0
	github.com/google/go-cmp v0.7.0
	github.com/jackc/pgx/v5 v5.11.0
	github.com/pganalyze/pg_query_go/v6 v6.2.2
	github.com/wasilibs/go-pgquery v0.0.0-20260915022521-81f99195012b
	go.yaml.in/yaml/v3 v3.0.5
	golang.org/x/tools v0.50.0
	google.golang.org/protobuf v1.36.12
	pgregory.net/rapid v1.3.0
)

require (
	github.com/go-openapi/jsonpointer v1.0.1 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/oasdiff/yaml v0.1.1 // indirect
	github.com/oasdiff/yaml3 v0.0.14 // indirect
	github.com/rogpeppe/go-internal v1.16.0 // indirect
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.3 // indirect
	github.com/tetratelabs/wazero v1.12.0 // indirect
	github.com/wasilibs/wazero-helpers v0.0.0-20250123031827-cd30c44769bb // indirect
	golang.org/x/mod v0.41.0 // indirect
	golang.org/x/sync v0.23.0 // indirect
	golang.org/x/sys v0.48.0 // indirect
	golang.org/x/text v0.42.0 // indirect
)

tool (
	github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/banproof
	github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/pgrun
	github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/vetcheck
)
