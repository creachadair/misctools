module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	github.com/DataDog/zstd v1.4.8 // indirect
	github.com/cockroachdb/errors v1.8.4 // indirect
	github.com/cockroachdb/redact v1.0.9 // indirect
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20210421124725-5714981793f8
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ctrl v0.1.0
	github.com/creachadair/ffs v0.0.0-20210330135354-d2fe618a7bf6
	github.com/creachadair/gcsstore v0.0.0-20210330140206-7c73a9488ad6
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/jrpc2 v0.13.0
	github.com/creachadair/keyfile v0.6.0
	github.com/creachadair/pebblestore v0.0.0-20210420135533-09b5435a8040
	github.com/creachadair/rpcstore v0.0.0-20210330135844-fc8fb1d6e6c8
	github.com/creachadair/sqlitestore v0.0.0-20210330140548-b8c83c455a73
	github.com/creachadair/vql v0.0.19
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/klauspost/compress v1.12.1 // indirect
	github.com/kr/pretty v0.2.1 // indirect
	github.com/tdewolff/minify/v2 v2.9.16
	github.com/tdewolff/parse/v2 v2.5.15 // indirect
	golang.org/x/crypto v0.0.0-20210421170649-83a5a9bb288b
	golang.org/x/exp v0.0.0-20210417010653-0739314eea07 // indirect
	golang.org/x/net v0.0.0-20210420210106-798c2154c571 // indirect
	google.golang.org/api v0.45.0 // indirect
	google.golang.org/genproto v0.0.0-20210421164718-3947dc264843 // indirect
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
