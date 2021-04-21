module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	github.com/DataDog/zstd v1.4.8 // indirect
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20210330140242-7924b90d3647
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ctrl v0.1.0
	github.com/creachadair/ffs v0.0.0-20210330135354-d2fe618a7bf6
	github.com/creachadair/gcsstore v0.0.0-20210330140206-7c73a9488ad6
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/jrpc2 v0.12.0
	github.com/creachadair/keyfile v0.6.0
	github.com/creachadair/pebblestore v0.0.0-20210420135533-09b5435a8040
	github.com/creachadair/rpcstore v0.0.0-20210212170421-ab45512f6769
	github.com/creachadair/sqlitestore v0.0.0-20210330140548-b8c83c455a73
	github.com/creachadair/vql v0.0.19
	github.com/tdewolff/minify/v2 v2.9.15
	golang.org/x/crypto v0.0.0-20210322153248-0c34fe9e7dc2
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
