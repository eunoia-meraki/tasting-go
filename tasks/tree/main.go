package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func dirTree(out io.Writer, path string, printFiles bool) error {

	err := filepath.WalkDir(path,
		func(walkPath string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}

			if path == walkPath {
				return nil
			}

			if !printFiles && !entry.IsDir() {
				return nil
			}

			relPath, err := filepath.Rel(path, walkPath)
			if err != nil {
				return err
			}
			split := strings.Split(relPath, string(os.PathSeparator))
			targPath := path

			for i := range split {
				entries, err := os.ReadDir(targPath)
				if err != nil {
					return err
				}

				if !printFiles {
					for !entries[len(entries)-1].IsDir() {
						entries = entries[:len(entries)-1]
					}
				}

				if i < len(split)-1 {
					if split[i] == entries[len(entries)-1].Name() {
						fmt.Fprint(out, "\t")
					} else {
						fmt.Fprint(out, "│\t")
					}
				} else {
					if entry.Name() == entries[len(entries)-1].Name() {
						fmt.Fprint(out, "└───")
					} else {
						fmt.Fprint(out, "├───")
					}
				}

				targPath = targPath + string(os.PathSeparator) + split[i]
			}

			fmt.Fprint(out, entry.Name())

			if !entry.IsDir() {
				info, err := entry.Info()
				if err != nil {
					return err
				}
				if size := info.Size(); size != 0 {
					fmt.Fprint(out, " (", size, "b)")
				} else {
					fmt.Fprint(out, " (empty)")
				}
			}

			fmt.Fprint(out, "\n")

			return nil
		})

	if err != nil {
		return err
	}

	return nil
}

func main() {
	out := os.Stdout
	if !(len(os.Args) == 2 || len(os.Args) == 3) {
		panic("usage go run main.go . [-f]")
	}
	path := os.Args[1]
	printFiles := len(os.Args) == 3 && os.Args[2] == "-f"
	err := dirTree(out, path, printFiles)
	if err != nil {
		panic(err.Error())
	}
}
