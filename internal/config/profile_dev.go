//go:build dev

package config

// Dev build profile, compiled in only with `go build -tags dev` (which is what
// `mise run dev` uses). Dev builds read a separate config file and store their
// token under a separate keyring entry, so iterating on the CLI never reads or
// clobbers the production credentials managed by an installed binary.
//
// None of this is compiled into release builds; see profile_prod.go.
const (
	configFileName     = "config.dev.yaml"
	apiKeyKeyringEntry = "api-key-dev"

	// IsDev reports whether this binary was built with `-tags dev`.
	IsDev = true
)
