package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"os"
)

const (
	Magic           = (('K' << 24) + ('C' << 16) + ('A' << 8) + 'P')
	HeaderLength    = 12
	FileBlockLength = 64
	FileNameLength  = 56
	FileOffset      = 56
	FileLength      = 60
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

	for i := 0; i < int(FileCount); i++ {
		File := PakFile{}
		_, e = f.Seek(int64(int(MetaOffset)+(i*FileBlockLength)), 0)
		Check(e)

		_, e = f.Read(block)
		Check(e)
		File.Name = ReadString(block, 0)
		File.Offset = int(ReadLong(block, FileOffset))
		File.Length = int(ReadLong(block, FileLength))
		fmt.Println(File)

		Files = append(Files, File)
	}

	_, e = f.Seek(int64(Files[0].Offset), 0)
	Check(e)

	fileblob := make([]byte, Files[0].Length)
	_, e = f.Read(fileblob)
	Check(e)

	f2, e := os.Create(Files[0].Name)
	Check(e)
	_, e = f2.Write(fileblob)
	Check(e)
	f2.Close()

	f.Close()
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
