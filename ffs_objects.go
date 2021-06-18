package main

type ffs_LocalFolder []string

func (i *ffs_LocalFolder) Set(value string) error {
	*i = append(*i, value)
	return nil
}
func (i *ffs_LocalFolder) String() string {
	return "my string representation"
}

type ffs_FilePart struct {
	data []byte
	id   int
}

type ffs_File struct {
	partcount int
	parts     []ffs_FilePart
}
