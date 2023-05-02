package main

import (
	"fmt"
	"io"
	"net"
)

func main() {
	var pool []*net.TCPConn
	var count int32
	count = 4
	srv, err := net.ListenTCP("tcp", &net.TCPAddr{Port: 6666})
	if err != nil {
		panic(err)
	}
	for i := int32(0); i < count; i++ {
		conn, err := net.DialTCP("tcp", nil, &net.TCPAddr{Port: 888})
		if err != nil {
			panic(err)
		}
		pool = append(pool, conn)
	}

	src, err := srv.AcceptTCP()
	if err != nil {
		panic(err)
	}

	idx := 0

	buffer := make([]byte, 512*1024)

	for {
		nr, er := src.Read(buffer)
		if nr > 0 {
			pool[idx].Write(buffer[:nr])
			idx++
			idx %= len(pool)
		}
		if er != nil {
			if er != io.EOF {
				fmt.Println(er)
			}
			break
		}
	}
}
