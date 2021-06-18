module github.com/nuveusltd/ffs.git

go 1.15

require (
	github.com/billziss-gh/cgofuse v1.5.0
	github.com/mattn/go-sqlite3 v1.14.7
	github.com/nuveusltd/nlib v0.0.0-00010101000000-000000000000
)

replace github.com/nuveusltd/nlib => ../nlib
