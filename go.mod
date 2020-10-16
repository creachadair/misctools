module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	cloud.google.com/go v0.69.1 // indirect
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20201016181728-d7a013eca088
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ffs v0.0.0-20201016181435-0d4dccf55695
	github.com/creachadair/gcsstore v0.0.0-20201015161812-5f1a8a28aaca
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/keyfile v0.5.3
	github.com/creachadair/sqlitestore v0.0.0-20201015161812-ae0b6346201c
	github.com/creachadair/vql v0.0.19
	github.com/golang/protobuf v1.4.3 // indirect
	github.com/tdewolff/minify/v2 v2.9.7
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/net v0.0.0-20201016165138-7b1cca2348c0 // indirect
	golang.org/x/sys v0.0.0-20201016160150-f659759dc4ca // indirect
	golang.org/x/tools v0.0.0-20201016155721-7c89c52d2f52 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201015140912-32ed001d685c // indirect
	google.golang.org/grpc v1.33.0 // indirect
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
