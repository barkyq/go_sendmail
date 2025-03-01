package main

import (
	"bufio"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/mail"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const conf_file = ".go_sendmail.conf"

var targetdir = flag.String("t", "mail/target", "target directory")
var portable = flag.Bool("p", false, "portable (not relative $HOME)")
var nosmtp = flag.Bool("nosmtp", false, "do not send on SMTP (put into local mail directory)")

func main() {
	flag.Parse()
	user_info := make(map[string]string)
	if s, e := os.UserHomeDir(); e == nil {
		if !*portable {
			*targetdir = filepath.Join(s, *targetdir)
		}
		userconfs := filepath.Join(s, conf_file)
		if f, e := os.Open(userconfs); e == nil {
			dec := json.NewDecoder(f)
			dec.Decode(&user_info)
		} else {
			panic(e)
		}
	} else {
		panic(e)
	}

	var raw_msg *os.File
	if f, e := os.CreateTemp("/tmp", "go_sendmail_tmp_"); e != nil {
		panic(e)
	} else if _, e := io.Copy(f, os.Stdin); e != nil {
		panic(e)
	} else if e := f.Close(); e != nil {
		panic(e)
	} else if g, e := os.Open(f.Name()); e != nil {
		panic(e)
	} else {
		raw_msg = g
	}

	msg, e := mail.ReadMessage(raw_msg)
	if e != nil {
		panic(e)
	}

	mechan := make(chan string)
	rp, wp := io.Pipe()
	go func() {
		var me string
		if a, e := mail.ParseAddress(msg.Header.Get("From")); e == nil {
			me = strings.ToLower(a.Address)
			if filename, ok := user_info[me]; ok != true {
				panic(ok)
			} else {
				if strings.HasSuffix(filename, ".gpg") {
					cmd := exec.Command("/usr/bin/gpg", "-qd", filename)
					cmd.Stdout = wp
					if e := cmd.Run(); e != nil {
						panic(fmt.Errorf("gpg decryption error!"))
					} else {
						wp.Close()
					}
				} else if f, e := os.Open(filename); e == nil {
					if _, e := io.Copy(wp, f); e != nil {
						panic(e)
					}
					wp.Close()
					f.Close()
				}
			}
			mechan <- me
		} else {
			panic(e)
		}
	}()
	hostname, port, a, e := LoadConfig(rp)
	if e != nil {
		panic(e)
	}
	rcpt_addrs := make([]*mail.Address, 0, 16)
	if a, e := msg.Header.AddressList("To"); e == nil {
		rcpt_addrs = append(rcpt_addrs, a...)
	}
	if a, e := msg.Header.AddressList("Cc"); e == nil {
		rcpt_addrs = append(rcpt_addrs, a...)
	}
	if a, e := msg.Header.AddressList("Bcc"); e == nil {
		rcpt_addrs = append(rcpt_addrs, a...)
	}

	// nobody handler
	if strings.SplitN(rcpt_addrs[0].Address, "@", 2)[0] == "nobody" {
		*nosmtp = true
	}

	// SMTP handler
	func() {
		if *nosmtp {
			fmt.Fprintln(os.Stderr, "no smtp")
			return
		}
		client, e := ClientConnect(a, hostname, port)
		if e != nil {
			panic(e)
		}
		if e := client.Mail(<-mechan); e != nil {
			panic(e)
		}
		done_addrs := make([]string, 0, len(rcpt_addrs))
		for _, addr := range rcpt_addrs {
			for _, d := range done_addrs {
				if d == addr.String() {
					continue
				}
			}
			if e := client.Rcpt(addr.Address); e != nil {
				panic(e)
			}
			done_addrs = append(done_addrs, addr.String())
		}
		data, e := client.Data()
		if e != nil {
			panic(e)
		}
		var wb *bufio.Writer
		if i, e := raw_msg.Stat(); e != nil {
			panic(e)
		} else {
			wb = bufio.NewWriterSize(data, int(i.Size()))
		}
		// msg.Body gets consumed
		if _, e := WriteMessage(msg.Header, msg.Body, wb); e != nil {
			panic(e)
		} else if e := wb.Flush(); e != nil {
			panic(e)
		} else if e := data.Close(); e != nil {
			panic(e)
		} else if e := client.Close(); e != nil {
			panic(e)
		} else if e := raw_msg.Close(); e != nil {
			panic(e)
		}
	}()

	// archive
	arch := &ArchiveTicket{
		hasher:   sha256.New(),
		header:   msg.Header,
		filename: raw_msg.Name(),
	}
	if name, e := arch.Submit(); e != nil {
		panic(e)
	} else {
		fmt.Println(name)
	}

	// notmuch
	msgid_query := fmt.Sprintf("id:%s", strings.Trim(msg.Header.Get("Message-ID"), "<>"))
	if e := exec.Command("notmuch", "new").Run(); e != nil {
		panic(e)
	} else if e := exec.Command("notmuch", "tag", "-unread", "+sent", msgid_query).Run(); e != nil {
		panic(e)
	}
}
