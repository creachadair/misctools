module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	cloud.google.com/go v0.76.0 // indirect
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20210103175220-1496fb259e86
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ctrl v0.1.0
	github.com/creachadair/ffs v0.0.0-20210203193155-101302c70704
	github.com/creachadair/gcsstore v0.0.0-20210103174737-c9faf956fb23
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/jrpc2 v0.11.2
	github.com/creachadair/keyfile v0.6.0
	github.com/creachadair/sqlitestore v0.0.0-20210103175203-055ac29f2c1f
	github.com/creachadair/vql v0.0.19
	github.com/tdewolff/minify/v2 v2.9.11
	go.opencensus.io v0.22.6 // indirect
	golang.org/x/crypto v0.0.0-20201221181555-eec23a3978ad
	golang.org/x/oauth2 v0.0.0-20210201163806-010130855d6c // indirect
	google.golang.org/genproto v0.0.0-20210203152818-3206188e46ba // indirect
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
