module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	cloud.google.com/go v0.73.0 // indirect
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20201108194349-10e56cb7e706
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ctrl v0.1.0
	github.com/creachadair/ffs v0.0.0-20201127193927-a8be2278214a
	github.com/creachadair/gcsstore v0.0.0-20201108194514-7100a1a9d112
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/jrpc2 v0.11.1
	github.com/creachadair/keyfile v0.6.0
	github.com/creachadair/sqlitestore v0.0.0-20201116175206-ab888adbd7f0
	github.com/creachadair/vql v0.0.19
	github.com/tdewolff/minify/v2 v2.9.10
	github.com/tdewolff/parse/v2 v2.5.6 // indirect
	golang.org/x/crypto v0.0.0-20201208171446-5f87f3452ae9
	golang.org/x/lint v0.0.0-20201208152925-83fdc39ff7b5 // indirect
	golang.org/x/net v0.0.0-20201207224615-747e23833adb // indirect
	golang.org/x/oauth2 v0.0.0-20201208152858-08078c50e5b5 // indirect
	golang.org/x/sync v0.0.0-20201207232520-09787c993a3a // indirect
	golang.org/x/sys v0.0.0-20201207223542-d4d67f95c62d // indirect
	golang.org/x/tools v0.0.0-20201208211828-de58e7c01d49 // indirect
	google.golang.org/genproto v0.0.0-20201207150747-9ee31aac76e7 // indirect
	google.golang.org/grpc v1.34.0 // indirect
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
