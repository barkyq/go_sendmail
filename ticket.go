package main

import (
	"bufio"
	"crypto/sha256"
	"fmt"
	"hash"
	"io"
	"net/mail"
	"os"
	"path/filepath"
)

const digest_length = 20

type ArchiveTicket struct {
	hasher   hash.Hash
	digest   [digest_length]byte
	filename string
	header   mail.Header
}

func (arch *ArchiveTicket) Submit() (name string, err error) {
	if e := arch.Hash(); e != nil {
		panic(e)
	}
	first_byte := fmt.Sprintf("%02x", arch.digest[0])
	rest_bytes := fmt.Sprintf("%02x", arch.digest[1:])
	name = filepath.Join(*targetdir, first_byte, rest_bytes)
	if i, e := os.Stat(filepath.Join(*targetdir, first_byte)); e != nil {
		if e := os.MkdirAll(filepath.Join(*targetdir, first_byte), os.ModePerm); e != nil {
			panic(e)
		}
	} else if !i.IsDir() {
		err = fmt.Errorf("invalid target directory structure")
	} else if outf, e := os.Create(name); e != nil {
		panic(e)
	} else if inf, e := os.Open(arch.filename); e != nil {
		panic(e)
	} else {
		rb := bufio.NewReader(inf)
		wb := bufio.NewWriter(outf)
		for {
			if ell, e := rb.ReadBytes('\r'); e != nil {
				if e == io.EOF {
					if len(ell) > 0 {
						if _, e := wb.Write(ell); e != nil {
							panic(e)
						}
					}
					break
				}
				panic(e)
			} else if _, e := wb.Write(ell[:len(ell)-1]); e != nil {
				panic(e)
			}
		}
		if e := wb.Flush(); e != nil {
			panic(e)
		} else if e := inf.Close(); e != nil {
			panic(e)
		} else if e := outf.Close(); e != nil {
			panic(e)
		}
	}
	return
}

// make sure hasher is reset before calling Hash()
func (a *ArchiveTicket) Hash() error {
	if a.hasher == nil {
		a.hasher = sha256.New()
	}
	if _, e := WriteHeaders(a.header, a.hasher); e != nil {
		return e
	} else {
		copy(a.digest[:], a.hasher.Sum(nil))
	}
	a.hasher.Reset()
	return nil
}
