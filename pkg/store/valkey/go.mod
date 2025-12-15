module github.com/codeGROOVE-dev/sfcache/pkg/store/valkey

go 1.25.4

require (
	github.com/codeGROOVE-dev/sfcache/pkg/store/compress v0.0.0
	github.com/valkey-io/valkey-go v1.0.69
)

require (
	github.com/klauspost/compress v1.18.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
)

replace github.com/codeGROOVE-dev/sfcache/pkg/store/compress => ../compress
