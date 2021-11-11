package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
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
	f, e := os.Open(os.Args[1])
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

	// write the files to the disk
	for i := range Files {
		WriteFile(&Files[i], f)
	}

	f.Close()
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
