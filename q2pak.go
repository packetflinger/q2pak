/**
 * q2pak can read and write Quake II .pak files.
 *
 * author: Joe Reid <claire@packetflinger.com>
 */

package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	Magic           = (('K' << 24) + ('C' << 16) + ('A' << 8) + 'P')
	HeaderLength    = 12
	FileBlockLength = 64 // name + offset + length
	FileNameLength  = 56
	FileOffset      = 56
	FileLength      = 60
	Separator       = "/"
)

/**
 * This represents a file inside a .pak file, not the .pak itself
 */
type PakFile struct {
	Name   string
	Offset int
	Length int
}

func main() {
	// parse the args
	List := flag.String("list", "", "list the files in the pak")
	Extract := flag.String("extract", "", "Extract all files from the pak")
	Create := flag.String("create", "", "Create a pack ")
	flag.Parse()

	if *Create != "" {
		sourcedir := flag.Arg(0)
		if sourcedir == "" {
			Usage()
			return
		}
		CreatePak(sourcedir, *Create)
	} else if *List != "" {
		Files := ParsePak(*List)
		ListFiles(Files)
	} else if *Extract != "" {
		Files := ParsePak(*Extract)
		ExtractFiles(Files, *Extract)
	} else {
		Usage()
	}
}

func Usage() {
	fmt.Println("Usage:")
	fmt.Printf("%s -list <pakfile>\n", os.Args[0])
	fmt.Printf("%s -extract <pakfile>\n", os.Args[0])
	fmt.Printf("%s -create <pakfile> <folder>\n", os.Args[0])
}

/**
 * Parse the file index to get a list of files in the pak
 * along with their locations and sizes
 *
 * Used for LISTing and EXTRACTing
 */
func ParsePak(pakfilename string) *[]PakFile {
	f, e := os.Open(pakfilename)
	Check(e)

	_, e = f.Seek(0, 0)
	Check(e)

	header := make([]byte, HeaderLength)
	_, e = f.Read(header)
	Check(e)

	Auth := ReadLong(header, 0)

	if Magic != Auth {
		panic(string("Invalid PAK file"))
	}

	// find the location and size of the meta lump
	MetaOffset := ReadLong(header, 4)
	MetaLength := ReadLong(header, 8)
	FileCount := MetaLength / FileBlockLength

	_, e = f.Seek(int64(MetaOffset), 0)
	Check(e)

	MetaBlock := make([]byte, MetaLength)
	_, e = f.Read(MetaBlock)
	Check(e)

	block := make([]byte, FileBlockLength)
	Files := []PakFile{}

	// build a slice of all the files contained and their size/locations
	for i := 0; i < int(FileCount); i++ {
		File := PakFile{}
		_, e = f.Seek(int64(int(MetaOffset)+(i*FileBlockLength)), 0)
		Check(e)

		_, e = f.Read(block)
		Check(e)

		File.Name = ReadString(block, 0)
		File.Offset = int(ReadLong(block, FileOffset))
		File.Length = int(ReadLong(block, FileLength))
		Files = append(Files, File)
	}

	f.Close()
	return &Files
}

/**
 * Dump all files in the pak to disk
 */
func ExtractFiles(files *[]PakFile, pak string) {
	f, e := os.Open(pak)
	Check(e)

	for _, file := range *files {
		WriteFile(&file, f)
	}
	f.Close()
}

/**
 * Just output the files contained in the pak
 */
func ListFiles(files *[]PakFile) {
	for _, file := range *files {
		fmt.Println(file.Name)
	}
}

/**
 * Make a new pak file from a directory in the filesystem
 */
func CreatePak(path string, newfile string) {
	var filenames []string
	var pakfiles []PakFile

	err := filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if info.IsDir() {
				return nil
			}

			filenames = append(filenames, path)
			return nil
		})
	Check(err)

	f2, err := os.Create(newfile)
	Check(err)

	// Write the 12 byte header
	_, err = f2.Write(WriteLong(Magic))
	Check(err)

	_, err = f2.Write(WriteLong(-1)) // placeholder, update later
	Check(err)

	_, err = f2.Write(WriteLong(-1)) // placeholder, update later
	Check(err)

	// write the actual file contents to the pak
	position := HeaderLength + 1
	for _, f := range filenames {
		file := PakFile{}
		fmt.Println(f)
		file.Name = f

		contents, err := os.ReadFile(file.Name)
		Check(err)

		file.Length = len(contents)
		b, err := f2.Write(contents)
		Check(err)

		file.Offset = position - 1
		position += b
		pakfiles = append(pakfiles, file)
	}

	// write the index
	for _, f := range pakfiles {
		name := make([]byte, FileNameLength)
		_ = copy(name, []byte(f.Name))

		_, err := f2.Write(name)
		Check(err)

		_, err = f2.Write(WriteLong(f.Offset))
		Check(err)

		_, err = f2.Write(WriteLong(f.Length))
		Check(err)
	}

	_, err = f2.Seek(int64(4), 0)
	Check(err)

	_, err = f2.Write(WriteLong(position - 1))
	Check(err)

	_, err = f2.Write(WriteLong(len(pakfiles) * FileBlockLength))
	Check(err)

	f2.Sync()
	f2.Close()
}

/**
 * Given the name, location and length, get the pak'd file contents
 * create the file in the file system (relative to where we are
 * when we run) and write the contents
 */
func WriteFile(file *PakFile, pak *os.File) {
	// pak'd file is in a folders (or multiple), create parent folders first
	if strings.Contains(file.Name, Separator) {
		pathparts := strings.Split(file.Name, Separator)
		path := pathparts[0] // to prevent starting with a separator
		for i := 1; i < len(pathparts)-1; i++ {
			path = fmt.Sprintf("%s%s%s", path, Separator, pathparts[i])
		}

		if _, err := os.Stat(path); os.IsNotExist(err) {
			os.MkdirAll(path, 0700)
		}
	}

	_, e := pak.Seek(int64(file.Offset), 0)
	Check(e)

	fileblob := make([]byte, file.Length)
	_, e = pak.Read(fileblob)
	Check(e)

	f2, e := os.Create(file.Name)
	Check(e)
	_, e = f2.Write(fileblob)
	Check(e)
	f2.Close()
}

/**
 * Convenience error checking
 */
func Check(e error) {
	if e != nil {
		panic(e)
	}
}

/**
 * Convert 4 bytes into an actual integer (little endian)
 */
func ReadLong(msg []byte, pos int) int32 {
	out := int32(msg[pos])
	out += int32(msg[pos+1]) << 8
	out += int32(msg[pos+2]) << 16
	out += int32(msg[pos+3]) << 24
	return out
}

/**
 * Turn an integer back into 4 bytes (little endian)
 */
func WriteLong(in int) []byte {
	out := make([]byte, 4)
	out[0] = byte(in & 255)
	out[1] = byte((in >> 8) & 255)
	out[2] = byte((in >> 16) & 255)
	out[3] = byte(in >> 24)
	return out
}

/**
 * basically just grab a subsection of the buffer
 */
func ReadData(msg []byte, pos int, length int) []byte {
	return msg[pos : pos+length]
}

/**
 * Keep building a string until we hit a null
 */
func ReadString(msg []byte, pos int) string {
	var buffer bytes.Buffer

	// find the next null (terminates the string)
	for i := pos; msg[i] != 0; i++ {
		if i == len(msg) {
			break
		}

		buffer.WriteString(string(msg[i]))
	}

	return buffer.String()
}
