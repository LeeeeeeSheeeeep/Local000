package main

import (
	"crypto/rand"
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: local000 <target_file>")
		os.Exit(1)
	}
	target := os.Args[1]
	shred(target)
}

func shred(path string) {
	info, err := os.Stat(path)
	if err != nil { return }
	
	f, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil { return }
	
	size := info.Size()
	zeros := make([]byte, size)
	
	// Pass 1-6: Random noise
	for i:=0; i<6; i++ {
		noise := make([]byte, size)
		rand.Read(noise)
		f.WriteAt(noise, 0)
		f.Sync()
	}
	
	// Pass 7: Zeros
	f.WriteAt(zeros, 0)
	f.Sync()
	f.Close()
	os.Remove(path)
	fmt.Println("Target physically annihilated.")
}
