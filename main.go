package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/mail"
	"net/smtp"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/oauth2"
)

const conf_file = ".go_sendmail.conf"

func main() {
	user_info := make(map[string]string)
	if s, e := os.UserHomeDir(); e == nil {
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
	var raw_msg io.Reader = os.Stdin

	msg, e := mail.ReadMessage(raw_msg)
	if e != nil {
		panic(e)
	}
	var me string
	user_config := make(map[string]string)
	if a, e := mail.ParseAddress(msg.Header.Get("From")); e == nil {
		me = strings.ToLower(a.Address)
		if filename, ok := user_info[me]; ok != true {
			panic(ok)
		} else {
			if strings.HasSuffix(filename, ".gpg") {
				cmd := exec.Command("/usr/bin/gpg", "-qd", filename)
				if read, e := cmd.StdoutPipe(); e == nil {
					done := make(chan error)
					go func() { done <- cmd.Run() }()
					dec := json.NewDecoder(read)
					dec.Decode(&user_config)
					if e := <-done; e != nil {
						panic(fmt.Errorf("gpg decryption error!"))
					}
				}
			} else if f, e := os.Open(filename); e == nil {
				dec := json.NewDecoder(f)
				dec.Decode(&user_config)
			}
		}
	} else {
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
	var client *smtp.Client
	var a smtp.Auth
	hostname := strings.Split(user_config["smtp_server"], ":")[0]
	port := strings.Split(user_config["smtp_server"], ":")[1]
	var header_list []string
	switch user_config["type"] {
	case "plain":
		a = smtp.PlainAuth("", user_config["user"], user_config["password"], hostname)
		// the originating mail client provides Message-ID
		// anticipating that the user is using emacs + notmuch do fcc
		header_list = []string{
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
		}
	case "gmail":
		// see https://developers.google.com/gmail/imap/imap-smtp#session_length_limits
		config := &oauth2.Config{
			ClientID:     user_config["clientid"],
			ClientSecret: user_config["clientsecret"],
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://accounts.google.com/o/oauth2/auth",
				TokenURL:  "https://oauth2.googleapis.com/token",
				AuthStyle: 0,
			},
			RedirectURL: "https://localhost",
			Scopes:      []string{"https://mail.google.com/"},
		}
		// the RefreshToken is really a *secret* piece of information
		// so do not share the source code!
		token := &oauth2.Token{
			TokenType:    "Bearer",
			RefreshToken: user_config["refreshtoken"],
		}
		a = XOAuth2(user_config["user"], config, token)
		// let gsmtp deal with the MessageID
		// the user should not use notmuch fcc with emacs
		// since gmail will automatically store a copy of the sent message
		header_list = []string{
			"From",
			"To",
			"Cc",
			"Subject",
			"In-Reply-To",
			"References",
			"Date",
			"MIME-Version",
			"Content-Type",
			"Content-Disposition",
			"Content-Transfer-Encoding",
		}
	case "outlook":
		config := &oauth2.Config{
			ClientID: user_config["clientid"],
			Endpoint: oauth2.Endpoint{
				AuthURL:   "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				TokenURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/token",
				AuthStyle: oauth2.AuthStyleAutoDetect,
			},
			RedirectURL: "https://localhost:8080",
			Scopes:      []string{"offline_access", "https://outlook.office365.com/IMAP.AccessAsUser.All", "https://outlook.office365.com/SMTP.Send"},
		}
		token := &oauth2.Token{
			TokenType:    "Bearer",
			RefreshToken: user_config["refreshtoken"],
		}
		a = XOAuth2(user_config["user"], config, token)
		header_list = []string{
			"From",
			"To",
			"Cc",
			"Subject",
			"In-Reply-To",
			"References",
			"Date",
			"MIME-Version",
			"Content-Type",
			"Content-Disposition",
			"Content-Transfer-Encoding",
		}
	}
	switch port {
	case "465":
		if conn, e := tls.Dial("tcp", user_config["smtp_server"], nil); e == nil {
			client, e = smtp.NewClient(conn, hostname)
			if e != nil {
				panic(e)
			}
			e = client.Auth(a)
			if e != nil {
				panic(e)
			}
		} else {
			panic(e)
		}
	case "587":
		if c, e := smtp.Dial(user_config["smtp_server"]); e == nil {
			if e := c.StartTLS(&tls.Config{ServerName: hostname}); e == nil {
				if e := c.Auth(a); e == nil {
					client = c
				} else {
					panic(e)
				}
			} else {
				panic(e)
			}
		} else {
			panic(e)
		}
	}
	if e := client.Mail(me); e != nil {
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
	for _, h := range header_list {
		if v := msg.Header.Get(h); v != "" {
			fmt.Fprintf(data, "%s: %s\n", h, v)
		}
	}
	fmt.Fprintf(data, "\n")
	io.Copy(data, msg.Body)

	data.Close()
	client.Close()

}

type xOAuth2 struct {
	useremail string
	config    *oauth2.Config
	token     *oauth2.Token
}

func XOAuth2(useremail string, config *oauth2.Config, token *oauth2.Token) smtp.Auth {
	return &xOAuth2{useremail, config, token}
}

func (a *xOAuth2) Start(server *smtp.ServerInfo) (string, []byte, error) {
	if !server.TLS {
		return "", nil, fmt.Errorf("unencrypted connection")
	}
	tsrc := (a.config).TokenSource(context.Background(), a.token)
	if t, err := tsrc.Token(); err == nil {
		str := fmt.Sprintf("user=%sauth=Bearer %s", a.useremail, t.AccessToken)
		resp := []byte(str)
		return "XOAUTH2", resp, nil
	} else {
		return "", []byte{}, err
	}
}

func (a *xOAuth2) Next(fromServer []byte, more bool) ([]byte, error) {
	if more {
		// We've already sent everything.
		return nil, fmt.Errorf("unexpected server challenge")
	}
	return nil, nil
}
