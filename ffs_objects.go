package main

import "time"

type ffs_LocalFolder []string

func (i *ffs_LocalFolder) Set(value string) error {
	*i = append(*i, value)
	return nil
}
func (i *ffs_LocalFolder) String() string {
	return "my string representation"
}

//parentid INTEGER,name TEXT, fsize INTEGER,isFolder bool,fullpath string,cdate datetime, mdate datetime,mode integer
type ffs_File struct {
	ID      int64
	Size    int64
	Name    string
	ModTime time.Time
	Mode    uint32
	Data    []byte
	DataEnc []byte
	Kind    int // 0 forWrite 1 forRead
}
