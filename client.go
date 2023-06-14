package main

import (
	"crypto/tls"
	"fmt"
	"net/smtp"
)

func ClientConnect(a smtp.Auth, hostname string, port string) (*smtp.Client, error) {
	client := new(smtp.Client)
	switch port {
	case "465":
		if conn, e := tls.Dial("tcp", fmt.Sprintf("%s:%s", hostname, port), nil); e == nil {
			client, e = smtp.NewClient(conn, hostname)
			if e != nil {
				return nil, e
			}
			e = client.Auth(a)
			if e != nil {
				return nil, e
			}
		} else {
			return nil, e
		}
	case "587":
		if c, e := smtp.Dial(fmt.Sprintf("%s:%s", hostname, port)); e == nil {
			if e := c.StartTLS(&tls.Config{ServerName: hostname}); e == nil {
				if e := c.Auth(a); e == nil {
					client = c
				} else {
					return nil, e
				}
			} else {
				return nil, e
			}
		} else {
			return nil, e
		}
	}
	return client, nil
}
