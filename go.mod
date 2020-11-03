module github.com/creachadair/misctools

go 1.13

require (
	bitbucket.org/creachadair/stringset v0.0.9
	cloud.google.com/go v0.71.0 // indirect
	github.com/creachadair/badgerstore v0.0.8
	github.com/creachadair/boltstore v0.0.0-20201103200826-fc531af6444d
	github.com/creachadair/command v0.0.0-20200910004628-e48505ecfece
	github.com/creachadair/ffs v0.0.0-20201103200711-69590edcdf03
	github.com/creachadair/gcsstore v0.0.0-20201103201159-ff5606cc01fd
	github.com/creachadair/getpass v0.1.1
	github.com/creachadair/keyfile v0.5.3
	github.com/creachadair/sqlitestore v0.0.0-20201103201230-c63a159c9d03
	github.com/creachadair/vql v0.0.19
	github.com/tdewolff/minify/v2 v2.9.7
	golang.org/x/crypto v0.0.0-20201016220609-9e8e0b390897
	golang.org/x/net v0.0.0-20201031054903-ff519b6c9102 // indirect
	golang.org/x/sys v0.0.0-20201101102859-da207088b7d1 // indirect
	golang.org/x/text v0.3.4 // indirect
	golang.org/x/tools v0.0.0-20201103190053-ac612affd56b // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20201103154000-415bd0cd5df6 // indirect
)

replace github.com/creachadair/sqlitestore => ../sqlitestore
