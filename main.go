package main

import (
	"bufio"
	"bytes"
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
	"time"

	imapclient "github.com/emersion/go-imap/client"
)

const conf_file = ".go_sendmail.conf"

var targetdir = flag.String("t", "mail/target", "target directory")
var portable = flag.Bool("p", false, "portable (not relative $HOME)")
var noimap = flag.Bool("noimap", false, "do not upload to IMAP when plain")
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
	hostname, port, a, imap_addr, imap_a, e := LoadConfig(rp)
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

	// SMTP handler
	func() {
		if strings.SplitN(rcpt_addrs[0].Address, "@", 2)[0] == "nobody" || *nosmtp {
			fmt.Fprintln(os.Stderr, "sending to nobody; no smtp")
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
		if _, e := WriteMessage(msg.Header, msg.Body, wb); e != nil {
			panic(e)
		} else if e := wb.Flush(); e != nil {
			panic(e)
		} else {
			data.Close()
			client.Close()
			raw_msg.Close()
		}
	}()

	// imap submission
	if imap_a != nil && !*noimap {
		if f, e := os.Open(raw_msg.Name()); e != nil {
			panic(e)
		} else if c, e := imapclient.DialTLS(imap_addr, nil); e != nil {
			panic(e)
		} else if e := c.Authenticate(imap_a); e != nil {
			panic(e)
		} else if stat, e := c.Select("sent", false); e != nil {
			panic(e)
		} else {
			defer c.Logout()
			buf := bytes.NewBuffer(nil)
			if _, e := io.Copy(buf, f); e != nil {
				panic(e)
			} else if e := c.Append(stat.Name, []string{"\\Seen"}, time.Now(), buf); e != nil {
				panic(e)
			} else if e := f.Close(); e != nil {
				panic(e)
			}
		}
	}

	// archive
	arch := &ArchiveTicket{
		hasher:  sha256.New(),
		batons:  nil,
		tickets: nil,
		file:    raw_msg,
		msg:     msg,
		rb:      nil,
		wb:      nil,
	}
	var new_name string
	if e := arch.Hash(); e != nil {
		panic(e)
	} else {
		first_byte := fmt.Sprintf("%02x", arch.digest[0])
		rest_bytes := fmt.Sprintf("%02x", arch.digest[1:])
		new_name = filepath.Join(*targetdir, first_byte, rest_bytes)
		if i, e := os.Stat(filepath.Join(*targetdir, first_byte)); e != nil {
			if e := os.MkdirAll(filepath.Join(*targetdir, first_byte), os.ModePerm); e != nil {
				panic(e)
			}
		} else if !i.IsDir() {
			panic("invalid target directory structure")
		}
		// TODO: rewrite to target directory and remove CRLFs (notmuch emacs does not like them)
		if e := os.Rename(raw_msg.Name(), new_name); e != nil {
			panic(e)
		} else {
			fmt.Println(new_name)
		}
	}
	msgid_query := fmt.Sprintf("id:%s", strings.Trim(msg.Header.Get("Message-ID"), "<>"))
	if e := exec.Command("notmuch", "new").Run(); e != nil {
		panic(e)
	} else if e := exec.Command("notmuch", "tag", "-unread", "+sent", msgid_query).Run(); e != nil {
		panic(e)
	}

}
