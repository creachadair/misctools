module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20210207181429-544d39a78261
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ctrl v0.1.0
	github.com/creachadair/ffs v0.0.0-20210207180800-4247e2ad1f50
	github.com/creachadair/gcsstore v0.0.0-20210207181815-fac811702b78
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/jrpc2 v0.11.2
	github.com/creachadair/keyfile v0.6.0
	github.com/creachadair/rpcstore v0.0.0-20210207181904-be6fee0a8479
	github.com/creachadair/sqlitestore v0.0.0-20210207181919-fea7d7c9fe78
	github.com/creachadair/vql v0.0.19
	github.com/tdewolff/minify/v2 v2.9.12
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
