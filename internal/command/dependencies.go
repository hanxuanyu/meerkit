package command

import (
	"io"
	"io/fs"
)

type Dependencies struct {
	Frontend fs.FS
	Stdin    io.Reader
	Stdout   io.Writer
	Stderr   io.Writer
	Version  string
}
