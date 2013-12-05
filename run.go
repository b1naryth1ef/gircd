package main

import "github.com/b1naryth1ef/gircd/gircd"

func main() {
	server := gircd.NewServer("6666", "")
	server.Start()
}
