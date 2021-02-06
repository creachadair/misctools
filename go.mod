module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20210206083712-994fbcba7ab6
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ctrl v0.1.0
	github.com/creachadair/ffs v0.0.0-20210203193155-101302c70704
	github.com/creachadair/gcsstore v0.0.0-20210206172114-f3fa62d6c271
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/jrpc2 v0.11.2
	github.com/creachadair/keyfile v0.6.0
	github.com/creachadair/sqlitestore v0.0.0-20210206172244-e9db02f51df2
	github.com/creachadair/vql v0.0.19
	github.com/tdewolff/minify/v2 v2.9.11
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
