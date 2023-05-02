package main

import (
	"fmt"
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
	idx2 := 0

	buffer := make([]byte, 512*1024)
	buffer2 := make([]byte, 512*1024)
	go func() {
		for {
			nr, er := pool[idx2].Read(buffer2)
			if nr > 0 {
				src.Write(buffer2[:nr])
				idx2++
				idx2 %= len(pool)
				fmt.Println(string(buffer2))
			}
			if er != nil {

			}
		}
	}()

	for {
		nr, er := src.Read(buffer)
		if nr > 0 {
			pool[idx].Write(buffer[:nr])
			idx++
			idx %= len(pool)
		}
		if er != nil {
			fmt.Println(er)
			//break
		}
	}
}
