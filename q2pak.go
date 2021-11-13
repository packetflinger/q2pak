package main

import (
	"bytes"
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const (
	Magic           = (('K' << 24) + ('C' << 16) + ('A' << 8) + 'P')
	HeaderLength    = 12
	FileBlockLength = 64
	FileNameLength  = 56
	FileOffset      = 56
	FileLength      = 60
	Separator       = "/"
)

type PakFile struct {
	Name   string
	Offset int
	Length int
}

func main() {
	// parse the args
	List := flag.Bool("list", false, "list the files in the pak")
	Extract := flag.Bool("extract", false, "Extract all files from the pak")
	Create := flag.String("create", "", "Create a pack ")
	flag.Parse()
	pakfilename := flag.Arg(0)

	if *Create != "" {
		CreatePak(*Create, pakfilename)
	} else {
		Files := ParsePak(pakfilename)
		if *List {
			ListFiles(Files)
		}

		if *Extract {
			ExtractFiles(Files, pakfilename)
		}
	}
}

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
		//fmt.Println(File)

		Files = append(Files, File)
	}

	f.Close()
	return &Files
}

func ExtractFiles(files *[]PakFile, pak string) {
	f, e := os.Open(pak)
	Check(e)
	for _, file := range *files {
		WriteFile(&file, f)
	}
	f.Close()
}

func ListFiles(files *[]PakFile) {
	for _, file := range *files {
		fmt.Println(file.Name)
	}
}

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

	//fmt.Printf("%s\n", hex.Dump(WriteLong(Magic)))

	f2, e := os.Create(newfile)
	Check(e)
	_, _ = f2.Write(WriteLong(Magic))
	_, _ = f2.Write(WriteLong(-1)) // placeholder
	_, _ = f2.Write(WriteLong(-1)) // placeholder
	f2.Sync()

	// write the actual file contents to the pak
	position := HeaderLength + 1
	for _, f := range filenames {
		file := PakFile{}
		fmt.Println(f)
		file.Name = f

		contents, err := os.ReadFile(file.Name)
		Check(err)
		file.Length = len(contents)
		b, e := f2.Write(contents)
		Check(e)
		file.Offset = position
		position += b
		pakfiles = append(pakfiles, file)
	}

	f2.Sync()
	// write the table of contents
	for _, f := range pakfiles {
		name := make([]byte, FileNameLength)
		_ = copy(name, []byte(f.Name))

		_, _ = f2.Write(name)
		_, _ = f2.Write(WriteLong(f.Offset))
		_, _ = f2.Write(WriteLong(f.Length))
	}

	_, e = f2.Seek(int64(4), 0)
	Check(e)

	_, _ = f2.Write(WriteLong(position - 1))
	_, _ = f2.Write(WriteLong(len(pakfiles) * FileBlockLength))

	f2.Sync()
	f2.Close()
}

/**
 * Given the name, location and length, get the pak'd file contents
 * create the file in the file system (relative to where we are
 * when we run) and write the contents
 */
func WriteFile(file *PakFile, pak *os.File) {
	// pak'd file is in a folded (or multiple), create parent folders first
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

func Check(e error) {
	if e != nil {
		panic(e)
	}
}

func ReadLong(msg []byte, pos int) int32 {
	var tmp struct {
		Value int32
	}

	r := bytes.NewReader(msg[pos : pos+4])
	if err := binary.Read(r, binary.LittleEndian, &tmp); err != nil {
		fmt.Println("binary.Read failed:", err)
	}

	//msg.Index += 4
	return tmp.Value
}

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
		// we hit the end without finding a null
		if i == len(msg) {
			break
		}

		buffer.WriteString(string(msg[i]))
	}

	return buffer.String()
}
