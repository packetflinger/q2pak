# Q2Pak
A command line utility for extracting, creating and listing files inside Quake 2 .pak files. Adding and removing individual files from a pak archive is not supported, but can be achieved simply by extracting the archive, making the file changes and recreating the archive.

# Usage
`# q2pak -list <filename.pak>`

`# q2pak -extract <filename.pak>`

`# q2pak -create <newpakname.pak> <directory to capture>`

# Compiling 
To build for your current system:

`# go build q2pak.go`

To build for another OS or ARCH (ex: 32bit windows):

`# GOOS=windows GOARCH=386 CGO_ENABLED=0 go build q2pak.go`

