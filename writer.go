package main

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"
)

// TODO: need to fix; cannot handle long headers (important for To:)
func WriteMessage(headers mail.Header, r io.Reader, w io.Writer) (n int, e error) {
	if a, e := mail.ParseAddress(headers.Get("From")); e == nil {
		if k, e := fmt.Fprintf(w, "From: %s\n", a.String()); e != nil {
			return n + k, e
		} else {
			n += k
		}
	}

	for _, s := range []string{"To", "Cc"} {
		buffer := bytes.NewBuffer(nil)
		if list, e := headers.AddressList(s); e != nil {
			if e == mail.ErrHeaderNotPresent {
				continue
			} else {
				return n, e
			}
		} else {
			for i, x := range list {
				if i > 0 {
					if k, e := buffer.Write([]byte{',', '\r', '\n', ' '}); e != nil {
						return n + k, e
					} else {
						n += k
					}
				}
				if k, e := buffer.WriteString(x.String()); e != nil {
					return n + k, e
				} else {
					n += k
				}
			}
		}
		if k, e := fmt.Fprintf(w, "%s: %s\n", s, buffer.Bytes()); e != nil {
			return n + k, e
		} else {
			n += k
		}
	}

	for _, h := range header_list {
		if v := headers.Get(h); v != "" {
			if len(v) > 500 {
				panic("header too long")
			}
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
	"Message-ID",
}

var header_list = []string{
	"Subject",
	"In-Reply-To",
	"References",
	"Date",
	"Message-ID",
	"MIME-Version",
	"Content-Type",
	"Content-Disposition",
	"Content-Transfer-Encoding",
}
