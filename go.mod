module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20201015023213-0eaede31bc9c
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ffs v0.0.0-20201015161306-487a09628da6
	github.com/creachadair/gcsstore v0.0.0-20201015023349-a1de43b38f70
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/keyfile v0.5.3
	github.com/creachadair/sqlitestore v0.0.0-20201015023425-d88f4f4de15f
	github.com/creachadair/vql v0.0.19
	github.com/tdewolff/minify/v2 v2.9.7
	golang.org/x/crypto v0.0.0-20201002170205-7f63de1d35b0
	golang.org/x/net v0.0.0-20201009032441-dbdefad45b89 // indirect
	golang.org/x/sys v0.0.0-20201009025420-dfb3f7c4e634 // indirect
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
