package main

import (
	"fmt"
	"io"
	"net/mail"
)

func WriteMessage(headers mail.Header, r io.Reader, w io.Writer) (n int, e error) {
	for _, h := range header_list {
		if v := headers.Get(h); v != "" {
			if k, e := fmt.Fprintf(w, "%s: %s\n", h, v); e != nil {
				return n + k, e
			} else {
				n += k
			}
		}
	}
	if r == nil {
		return
	}
	w.Write([]byte{'\n'})
	n++
	if k, e := io.Copy(w, r); e != nil {
		return n + int(k), e
	} else {
		return n + int(k), nil
	}
}

func WriteHeaders(headers mail.Header, w io.Writer) (n int, e error) {
	for _, h := range canonical_header_list {
		if v := headers.Get(h); v != "" {
			if k, e := fmt.Fprintf(w, "%s: %s\n", h, v); e != nil {
				return n + k, e
			} else {
				n += k
			}
		}
	}
	return n, nil
}

var canonical_header_list = []string{
	"From",
	"Date",
	"Message-ID",
}

var header_list = []string{
	"From",
	"To",
	"Cc",
	"Subject",
	"In-Reply-To",
	"References",
	"Date",
	"Message-ID",
	"MIME-Version",
	"Content-Type",
	"Content-Disposition",
	"Content-Transfer-Encoding",
	"Hash",
}
