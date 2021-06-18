package main

import (
	"database/sql"
	"encoding/binary"
	"flag"
	"fmt"
	"log"
	"math"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/billziss-gh/cgofuse/fuse"
	_ "github.com/mattn/go-sqlite3"
	"github.com/nuveusltd/nlib"
)

const (
	fsBlockSize = 4096
)

var (
	BuildNumber string
	Version     string
	enckey      []byte
)

type ffs struct {
	fuse.FileSystemBase
	DB       *sql.DB
	folders  []string
	csFolder string
	uid      uint32
	gid      uint32
}

func usage() {
	fmt.Println("ffs FileSytem " + Version + "." + BuildNumber)
	flag.PrintDefaults()
}

// Init is called when the file system is created.
func (fs *ffs) Init() {
	log.Printf("Init Called \n")
}

// Destroy is called when the file system is destroyed.
func (fs *ffs) Destroy() {
	log.Printf("Destroy Called \n")
}

// Statfs gets file system statistics.
func (fs *ffs) Statfs(path string, stat *fuse.Statfs_t) int {

	stat.Fsid = 1111111111

	stat.Bsize = fsBlockSize
	stat.Blocks = 10000000
	stat.Bavail = 9000000
	stat.Bfree = 9000000
	stat.Favail = 900
	stat.Ffree = 900
	stat.Files = 1000
	stat.Frsize = 1
	stat.Namemax = 1000
	return 0
}

func (fs *ffs) findPathID(pathstr string) int {
	pathstr = filepath.Dir(pathstr)
	if pathstr == "/" || pathstr == "." {
		return -1
	}
	var id int = -1
	fs.DB.QueryRow("select rowid from items where fullpath=?", pathstr).Scan(&id)
	return id
}

func (fs *ffs) getFolder4id(rowid uint64) string {
	i := int(rowid)
	i = int(math.Trunc(float64(i) / 256))
	if i < 256*16 {
		return fmt.Sprintf("/%03s", fmt.Sprintf("%X", i))
	}
	oni := i - (256 * 16) + 1
	i = i - oni
	oni = int(math.Trunc(float64(oni) / 256))

	return fmt.Sprintf("/%03s/%03s", fmt.Sprintf("%X", oni), fmt.Sprintf("%X", i))
}

func (fs *ffs) checkFolder(s string) {
	for _, path := range fs.folders {
		fullpath := filepath.Join(path, s)
		if _, err := os.Stat(fullpath); os.IsNotExist(err) {
			os.MkdirAll(fullpath, 0700)
		}
	}
	fullpath := filepath.Join(fs.csFolder, s)
	if _, err := os.Stat(fullpath); os.IsNotExist(err) {
		os.MkdirAll(fullpath, 0700)
	}
}

func (fs *ffs) createFileName(rowid uint64) string {
	folder := fs.getFolder4id(rowid)
	fs.checkFolder(folder)
	return filepath.Join(folder, fmt.Sprintf("%03d", rowid))
}

func (fs *ffs) appendFile(filename string, bytes []byte) error {
	f, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return err
	}
	if _, err := f.Write(bytes); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return nil
}

func (fs *ffs) readParts(filename string, ofset int) map[int][]byte {
	result := make(map[int][]byte)
	blockIndex := math.Ceil(float64(ofset) / float64(fsBlockSize))

	/*readLen := ofset % fsBlockSize
	if readLen == 0 {
		readLen = fsBlockSize
	}
	buff := make([]byte, readLen) */

	bsizebuff := make([]byte, 10)
	var bsize uint64
	for i, path := range fs.folders {
		fullpath := filepath.Join(path, fmt.Sprintf("%s.dat%d", filename, i))
		fmt.Printf("- %s", fullpath)
		f, _ := os.OpenFile(fullpath, os.O_RDONLY, os.ModePerm)
		currentBlock := 0
		for {
			f.Read(bsizebuff)
			fmt.Printf("- %v", bsizebuff)
			bsize, _ = binary.Uvarint(bsizebuff)
			fmt.Printf("- block %d\n", bsize)
			if currentBlock == int(blockIndex) {
				blockforRead := make([]byte, bsize)
				f.Read(blockforRead)
				result[i+1] = blockforRead
				break
			} else {
				f.Seek(int64(bsize), 1)
			}
			currentBlock++
		}
	}
	fullpath := filepath.Join(fs.csFolder, fmt.Sprintf("%s.sum", filename))
	f, _ := os.OpenFile(fullpath, os.O_RDONLY, os.ModePerm)
	currentBlock := 0
	for {
		f.Read(bsizebuff)
		bsize, _ = binary.Uvarint(bsizebuff)
		if currentBlock == int(blockIndex) {
			blockforRead := make([]byte, bsize)
			f.Read(blockforRead)
			result[0] = blockforRead
			break
		} else {
			f.Seek(int64(bsize), 1)
		}
		currentBlock++
	}
	return result
}

func (fs *ffs) truncateFile(filename string) {
	for i, path := range fs.folders {
		fullpath := filepath.Join(path, fmt.Sprintf("%s.dat%d", filename, i))
		f, _ := os.OpenFile(fullpath, os.O_RDWR, 0666)
		defer f.Close()
		f.Truncate(0)
	}
	f, _ := os.OpenFile(filepath.Join(fs.csFolder, filename+".sum"), os.O_RDWR, 0666)
	defer f.Close()
	f.Truncate(0)
}

/*
// Mknod creates a file node.
func (fs *ffs) Mknod(path string, mode uint32, dev uint64) int {
	log.Printf("Mknod Called \n")
	return 0
}
*/

// Mkdir creates a directory.
func (fs *ffs) Mkdir(path string, mode uint32) int {
	_, e := fs.DB.Exec("insert into items(parentid,name,fsize,isFolder,fullpath,cdate,mdate) VALUES (?,?,?,?,?,?,?)", fs.findPathID(path), filepath.Base(path), 0, true, path, time.Now(), time.Now())
	if e != nil {
		log.Println(e)
	}
	return 0
}

// Unlink removes a file.
func (fs *ffs) Unlink(path string) int {
	log.Printf("Unlink Called \n")
	var rowid int
	fs.DB.QueryRow("select rowid from items where fullpath=? and isFolder=false", path).Scan(&rowid)

	fs.DB.Exec("delete from items where rowid=?", rowid)
	filename := fs.createFileName(uint64(rowid))
	for i, path := range fs.folders {
		os.Remove(filepath.Join(path, fmt.Sprintf("%s.dat%d", filename, i)))
	}
	return 0
}

// Rmdir removes a directory.
func (fs *ffs) Rmdir(path string) int {
	fs.DB.Exec("delete from items where fullpath=? and isFolder=true", path)
	return 0
}

// Link creates a hard link to a file.
func (fs *ffs) Link(oldpath string, newpath string) int {
	log.Printf("Link Called \n")
	return 0
}

// Symlink creates a symbolic link.
func (fs *ffs) Symlink(target string, newpath string) int {
	log.Printf("Symlink Called \n")
	return 0
}

// Readlink reads the target of a symbolic link.
func (fs *ffs) Readlink(path string) (int, string) {
	log.Printf("Readlink Called \n")
	return 0, ""
}

// Rename renames a file.
func (fs *ffs) Rename(oldpath string, newpath string) int {
	fs.DB.Exec("update items set name=?,fullpath=?,parentid=? where fullpath=?", filepath.Base(newpath), newpath, fs.findPathID(newpath), oldpath)
	return 0
}

// Chmod changes the permission bits of a file.
func (fs *ffs) Chmod(path string, mode uint32) int {
	log.Printf("Chmod Called \n")
	fs.DB.Exec("update items set mode=? where fullpath=?", mode, path)
	return 0
}

/*
// Chown changes the owner and group of a file.
func (fs *ffs) Chown(path string, uid uint32, gid uint32) int {
	log.Printf("Chown Called \n")
	return 0
}

// Utimens changes the access and modification times of a file.
func (fs *ffs) Utimens(path string, tmsp []fuse.Timespec) int {
	log.Printf("Utimens Called \n")
	return 0
}

*/
// Open opens a file.
// The flags are a combination of the fuse.O_* constants.
func (fs *ffs) Open(path string, flags int) (int, uint64) {
	log.Printf(nlib.BashFontColor_GREEN+"Open Called %s FLAG: %d \n"+nlib.BashFontColor_RESET, path, flags)
	var rowid uint64
	err := fs.DB.QueryRow("select rowid from items where fullpath=?", path).Scan(&rowid)
	if err != nil {
		fmt.Printf("err")
		return -fuse.ENOENT, 0 //No such file or directory
	}
	return 0, rowid
}

// Getattr gets file attributes.
func (fs *ffs) Getattr(path string, stat *fuse.Stat_t, fh uint64) int {
	fmt.Printf(nlib.BashFontColor_RED+"Getattr Called  %s \n"+nlib.BashFontColor_RESET, path)
	var rowid uint64
	if path == "/" || path == "." || path == ".." {
		stgo := syscall.Stat_t{}
		path := filepath.Join(fs.folders[0])
		syscall.Lstat(path, &stgo)
		copyFusestatFromGostat(stat, &stgo)
	} else {
		var fsize int64
		var isFolder bool
		if ^uint64(0) == fh {
			err := fs.DB.QueryRow("select fsize,isFolder,rowid from items where fullpath=?", path).Scan(&fsize, &isFolder, &rowid)
			if err != nil {
				fmt.Printf("err")
				return -fuse.ENOENT //No such file or directory
			}
		} else {
			rowid = fh
			err := fs.DB.QueryRow("select fsize,isFolder from items where rowid=?", fh).Scan(&fsize, &isFolder)
			if err != nil {
				fmt.Printf("err")
				return -fuse.ENOENT //No such file or directory
			}
		}
		stat.Ino = rowid
		stat.Gid = fs.gid
		stat.Uid = fs.uid
		stat.Atim = fuse.NewTimespec(time.Now())
		stat.Mtim = stat.Atim
		if isFolder {
			stat.Mode = 16877
		} else {
			stat.Mode = 33206 //fuse.S_IFREG | 0444
			stat.Size = fsize
			stat.Blksize = 4096
			stat.Blocks = 1
		}
	}
	//fmt.Printf("%#v \n", stat)
	return 0
}

// Read reads data from a file.
func (fs *ffs) Read(path string, buff []byte, ofst int64, fh uint64) int {
	log.Printf(nlib.BashFontColor_YELLOW+"Read Called %s offset %d fh %d \n"+nlib.BashFontColor_RESET, path, ofst, fh)

	parts := fs.readParts(fs.createFileName(fh), int(ofst))
	part1 := nlib.Decrypt(parts[1], enckey)
	part2 := nlib.Decrypt(parts[2], enckey)
	return copy(buff, append(part1, part2...))
}

// Truncate changes the size of a file.
func (fs *ffs) Truncate(path string, size int64, fh uint64) int {
	log.Printf("Truncate Called %s, size:%d, rec:%d \n", path, size, fh)
	_, err := fs.DB.Exec("update items set fsize=? where rowid=?", size, fh)
	if err != nil {
		fmt.Printf("err")
	}
	filename := fs.createFileName(fh)
	fs.truncateFile(filename)
	return 0
}

// Create creates and opens a file.
// The flags are a combination of the fuse.O_* constants.
func (fs *ffs) Create(path string, flags int, mode uint32) (errc int, fh uint64) {
	log.Printf("Create called %s flags : %d , mode : %d \n", path, flags, mode)
	res, e := fs.DB.Exec("insert into items(parentid,name,fsize,isFolder,fullpath,cdate,mdate) VALUES (?,?,?,?,?,?,?)", fs.findPathID(path), filepath.Base(path), 0, false, path, time.Now(), time.Now())
	if e != nil {
		log.Println(e)
	}
	fhi, _ := res.LastInsertId()
	return 0, uint64(fhi)
}

// Write writes data to a file.
func (fs *ffs) Write(path string, buff []byte, ofst int64, fh uint64) int {
	//log.Printf(nlib.BashFontColor_YELLOW+"Write Called ofst:%d,bsize:%d  \n"+nlib.BashFontColor_RESET, ofst, len(buff))

	size := float64(len(buff))
	partsize := int(math.Ceil(size / float64(len(fs.folders))))
	filename := fs.createFileName(fh)
	bs := make([]byte, 10)
	for i, folder := range fs.folders {
		toWrite := nlib.Encrypt(buff[i*partsize:(i+1)*partsize], enckey)
		binary.PutUvarint(bs, uint64(len(toWrite)))
		toWrite = append(bs, toWrite...)
		fs.appendFile(filepath.Join(folder, fmt.Sprintf("%s.dat%d", filename, i)), toWrite)
	}

	csum := make([]byte, partsize)

	for i := 0; i < len(fs.folders); i++ {
		if i == 0 {
			csum = buff[i*partsize : (i+1)*partsize]
		} else {
			csum = nlib.XOR2Bytes(csum, buff[i*partsize:(i+1)*partsize])
		}
	}
	binary.PutUvarint(bs, uint64(len(csum)))
	toWrite := append(bs, csum...)
	fs.appendFile(filepath.Join(fs.csFolder, fmt.Sprintf("%s.sum", filename)), toWrite)
	fs.DB.Exec("update items set fsize=fsize+? where rowid=?", size, fh)
	return len(buff)

}

// Flush flushes cached file data.
func (fs *ffs) Flush(path string, fh uint64) int {
	log.Printf("Flush Called %s %d \n", path, fh)
	return 0
}

// Release closes an open file.
func (fs *ffs) Release(path string, fh uint64) int {
	log.Printf("Release Called \n")
	return 0
}

// Fsync synchronizes file contents.
func (fs *ffs) Fsync(path string, datasync bool, fh uint64) int {
	log.Printf("Fsync Called \n")
	return 0
}

// Opendir opens a directory.
func (fs *ffs) Opendir(path string) (int, uint64) {
	log.Printf("Opendir Called \n")
	return 0, 1
}

// Readdir reads a directory.
func (fs *ffs) Readdir(path string,
	fill func(name string, stat *fuse.Stat_t, ofst int64) bool,
	ofst int64,
	fh uint64) int {
	log.Printf("Readdir Called \n")
	fill(".", nil, 0)
	fill("..", nil, 0)

	var rows *sql.Rows
	if path == "/" {
		log.Println("Root folder read")
		rows, _ = fs.DB.Query("SELECT name FROM items WHERE parentid=-1")
	} else {
		rows, _ = fs.DB.Query("SELECT name FROM items WHERE parentid=(select rowid from items where fullpath=?)", path)
	}
	defer rows.Close()
	i := 0
	for rows.Next() {
		log.Print(".")
		i++
		var n string
		rows.Scan(&n)
		fill(n, nil, 0)
	}
	log.Printf(" %d items found\n", i)
	return 0
}

// Releasedir closes an open directory.
func (fs *ffs) Releasedir(path string, fh uint64) int {
	log.Printf("Releasedir Called \n")
	return 0
}

// Fsyncdir synchronizes directory contents.
func (fs *ffs) Fsyncdir(path string, datasync bool, fh uint64) int {
	log.Printf("Fsyncdir Called \n")
	return 0
}

// Setxattr sets extended attributes.
func (fs *ffs) Setxattr(path string, name string, value []byte, flags int) int {
	log.Printf("Setxattr Called \n")
	return 0
}

// Getxattr gets extended attributes.
func (fs *ffs) Getxattr(path string, name string) (int, []byte) {
	log.Printf("Getxattr Called \n")
	return 0, []byte("")
}

// Removexattr removes extended attributes.
func (fs *ffs) Removexattr(path string, name string) int {
	log.Printf("Removexattr Called \n")
	return 0
}

// Listxattr lists extended attributes.
func (fs *ffs) Listxattr(path string, fill func(name string) bool) int {
	log.Printf("Listxattr \n")
	return 0
}

//Creates Empty SQLiteDB
func (fs *ffs) CreateDb() {
	fs.DB, _ = sql.Open("sqlite3", filepath.Join(fs.folders[0], "/.mfs_db"))
	fs.DB.Exec("CREATE TABLE IF NOT EXISTS items (parentid INTEGER,name TEXT, fsize INTEGER,isFolder bool,fullpath string,cdate datetime, mdate datetime,mode integer)")
	fs.DB.Exec("CREATE INDEX IF NOT EXISTS ix_items_parentid ON items(parentid)")
	fs.DB.Exec("CREATE INDEX IF NOT EXISTS ix_items_fullpath ON items(fullpath)")
}

func init() {
	log.SetFlags(0)
	log.SetPrefix("ffs : ")
	log.SetFlags(log.Ldate | log.Lmicroseconds)
}

func main() {
	enckey = []byte(nlib.GetMD5Hash("--ffs2021.06.21MFS"))
	syscall.Unmount("/Users/fatih/tmp/testFolder/mp", 1)

	fmt.Println(nlib.Decrypt(nlib.Encrypt([]byte("ffs FileSystem"), enckey), enckey))

	var mountPoint string
	var checksumdir string
	var dataFolders ffs_LocalFolder

	flag.StringVar(&mountPoint, "mountpoint", "", "Mount Folder")
	flag.StringVar(&checksumdir, "checksumdir", "", "CheckSum Store Folder")
	flag.Var(&dataFolders, "source", "Data Store Folder")
	flag.Parse()

	if len(dataFolders) < 2 {
		log.Fatal("You must enter minimum 2 sources")
	}
	if len(mountPoint) < 1 {
		log.Fatal("You must enter mountpoint")
	}

	u, _ := user.Current()
	gid, _ := strconv.Atoi(u.Gid)
	uid, _ := strconv.Atoi(u.Uid)
	fs := ffs{gid: uint32(gid), uid: uint32(uid), csFolder: checksumdir, folders: dataFolders}

	log.Printf("%#v", fs)

	dbfile := filepath.Join(fs.folders[0], "/.mfs_db")
	if _, err := os.Stat(dbfile); os.IsNotExist(err) {
		fs.CreateDb()
	} else {
		fs.DB, err = sql.Open("sqlite3", dbfile)
		if err != nil {
			log.Fatalf("Database Error 1003: %s\n", err)
		}
	}

	_host := fuse.NewFileSystemHost(&fs)
	_host.Mount(mountPoint, []string{}) // []string{"-d"})
}
