module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	crawshaw.io/sqlite v0.3.3-0.20210317204950-23d646f8ac00 // indirect
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20210508190944-47f42dfc7342
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ctrl v0.1.0
	github.com/creachadair/ffs v0.0.0-20210330135354-d2fe618a7bf6
	github.com/creachadair/gcsstore v0.0.0-20210508191126-7de3adf9e5cb
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/jrpc2 v0.14.0
	github.com/creachadair/keyfile v0.6.0
	github.com/creachadair/pebblestore v0.0.0-20210508191318-c7d35231b37b
	github.com/creachadair/rpcstore v0.0.0-20210510033421-38296c60a7e9
	github.com/creachadair/sqlitestore v0.0.0-20210330140548-b8c83c455a73
	github.com/creachadair/vql v0.0.19
	github.com/tdewolff/minify/v2 v2.9.16
	github.com/tdewolff/parse/v2 v2.5.16 // indirect
	golang.org/x/crypto v0.0.0-20210506145944-38f3c27a63bf
	golang.org/x/lint v0.0.0-20210508222113-6edffad5e616 // indirect
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
