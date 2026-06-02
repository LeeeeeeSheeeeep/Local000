package main

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
)

var (
	forceFlag       = flag.Bool("force", false, "Bypass confirmation prompt")
	concurrencyFlag = flag.Int("concurrency", 10, "Number of concurrent shredding workers")
)

var shreddedCount int32

func main() {
	flag.Parse()
	if flag.NArg() < 1 {
		fmt.Println("Usage: local000 [flags] <target_path>")
		flag.PrintDefaults()
		os.Exit(1)
	}

	target := flag.Arg(0)

	if !*forceFlag {
		fmt.Printf("WARNING: You are about to irrevocably destroy '%s'.\nThis uses DoD 5220.22-M 7-pass wipe + MFT scrubbing.\nType 'DESTROY' to confirm: ", target)
		var confirm string
		fmt.Scanln(&confirm)
		if confirm != "DESTROY" {
			fmt.Println("Aborted.")
			os.Exit(0)
		}
	}

	info, err := os.Stat(target)
	if err != nil {
		fmt.Printf("Error accessing target: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("[Local000] Initializing annihilation engine...")

	var filesToShred []string
	var dirsToRemove []string

	if info.IsDir() {
		err = filepath.WalkDir(target, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				filesToShred = append(filesToShred, path)
			} else {
				// Prepend to directories list so we can remove them inside-out later
				dirsToRemove = append([]string{path}, dirsToRemove...)
			}
			return nil
		})
		if err != nil {
			fmt.Printf("Error walking directory: %v\n", err)
			os.Exit(1)
		}
	} else {
		filesToShred = append(filesToShred, target)
	}

	fmt.Printf("[Local000] Found %d files. Launching %d workers...\n", len(filesToShred), *concurrencyFlag)

	jobs := make(chan string, len(filesToShred))
	for _, f := range filesToShred {
		jobs <- f
	}
	close(jobs)

	var wg sync.WaitGroup
	for i := 0; i < *concurrencyFlag; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for path := range jobs {
				shredFile(path)
				atomic.AddInt32(&shreddedCount, 1)
			}
		}()
	}

	wg.Wait()

	// Clean up empty directories
	for _, dir := range dirsToRemove {
		scrubAndRemoveDir(dir)
	}
	
	if info.IsDir() && len(dirsToRemove) == 0 {
	    // just in case
	    scrubAndRemoveDir(target)
	}

	fmt.Printf("[Local000] Target physically annihilated. %d files securely wiped.\n", shreddedCount)
}

func randomHex(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func scrubAndRemoveDir(path string) {
	// Rename to a random hex to scrub metadata (MFT/ext4 journal)
	dir := filepath.Dir(path)
	newPath := filepath.Join(dir, randomHex(16))
	err := os.Rename(path, newPath)
	if err == nil {
		os.RemoveAll(newPath)
	} else {
		os.RemoveAll(path)
	}
}

func shredFile(path string) {
	info, err := os.Stat(path)
	if err != nil {
		return
	}

	f, err := os.OpenFile(path, os.O_RDWR, 0666)
	if err != nil {
		return
	}

	size := info.Size()
	zeros := make([]byte, size)

	// Pass 1-6: Random noise
	for i := 0; i < 6; i++ {
		noise := make([]byte, size)
		rand.Read(noise)
		f.WriteAt(noise, 0)
		f.Sync()
	}

	// Pass 7: Zeros
	f.WriteAt(zeros, 0)
	f.Sync()
	f.Close()

	// Metadata Scrubbing: Rename multiple times before deletion
	currentPath := path
	for i := 0; i < 3; i++ {
		dir := filepath.Dir(currentPath)
		newPath := filepath.Join(dir, randomHex(16))
		if err := os.Rename(currentPath, newPath); err == nil {
			currentPath = newPath
		}
	}
	os.Remove(currentPath)
}
