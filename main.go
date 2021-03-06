package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/alecholmes/tdiff/lib"
)

var (
	packageFlag = flag.String("package", "", "Package to find reachable diff from")
	shaFlag     = flag.String("sha", "", "Git SHA")

	// Output format flags
	packagesFlag = flag.Bool("packages", false, "If set, all relevant changed packages printed")
	filesFlag    = flag.Bool("files", false, "If set, all relevant changed files are printed")
	commitsFlag  = flag.Bool("commits", false, "If set, all relevant commits are printed")
)

func main() {
	flag.Parse()
	if len(*packageFlag) == 0 || len(*shaFlag) == 0 {
		flag.Usage()
		os.Exit(1)
	}

	// Should od XOR check on output flags. Exactly one must be set.
	// if !*packagesFlag && !*filesFlag && !*commitsFlag {
	// 	log.Fatal("At least one output flag must be specified")
	// }

	packageNamer, err := newFullPackageNamer()
	if err != nil {
		log.Fatal(err)
	}

	files, err := lib.DiffFiles(*shaFlag, "HEAD")
	if err != nil {
		log.Fatal(err)
	}

	changedPackageFiles := make(map[string][]string)
	for _, file := range files {
		if strings.HasSuffix(file, ".go") {
			packageName := packageNamer(filepath.Dir(file))
			changedPackageFiles[packageName] = append(changedPackageFiles[packageName], file)
		}
	}

	reachablePackages, err := recursiveDeps(*packageFlag)
	if err != nil {
		log.Fatal(err)
	}

	relevantPackages := make(stringSet)
	for _, pckage := range reachablePackages {
		if _, ok := changedPackageFiles[pckage]; ok {
			relevantPackages.add(pckage)
		}
	}

	if *packagesFlag {
		for pckage := range relevantPackages {
			fmt.Println(pckage)
		}
	}

	if *filesFlag {
		for pckage := range relevantPackages {
			fmt.Println(strings.Join(changedPackageFiles[pckage], "\n"))
		}
	}

	if *commitsFlag {
		commits, err := lib.Commits(*shaFlag, "HEAD")
		if err != nil {
			log.Fatal(err)
		}

		relevantFiles := make(stringSet)
		for pckage := range relevantPackages {
			relevantFiles.add(changedPackageFiles[pckage]...)
		}

		var relevantCommits []lib.GitCommit
		for _, commit := range commits {
			files, err := lib.CommitFiles(commit.SHA)
			if err != nil {
				log.Fatal(err)
			}

			relevant := false
			for _, file := range files {
				if relevantFiles.contains(file) {
					relevant = true
					break
				}
			}
			if relevant {
				relevantCommits = append(relevantCommits, commit)
			}
		}

		for _, commit := range relevantCommits {
			fmt.Printf("%s %s\n", commit.SHA, commit.Description)
		}
	}
}

type stringSet map[string]bool

func (s stringSet) add(values ...string) {
	for _, v := range values {
		s[v] = true
	}
}

func (s stringSet) contains(value string) bool {
	return s[value]
}

type fullPackagerNamer func(relativePackage string) string

func newFullPackageNamer() (fullPackagerNamer, error) {
	srcDir := fmt.Sprintf("%s/src/", os.Getenv("GOPATH"))
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	if !strings.HasPrefix(wd, srcDir) {
		return nil, fmt.Errorf("Expected working directoring to be under %s; working directory is %s", srcDir, wd)
	}

	packagePrefix := wd[len(srcDir):]

	return func(relativePackage string) string {
		// TODO: check for GOROOT builtins before prefixing
		return fmt.Sprintf("%s/%s", packagePrefix, relativePackage)
	}, nil
}

func recursiveDeps(root string) ([]string, error) {
	out, err := lib.RunCommand("go", "list", "-f", "'{{.Deps}}'", root)
	if err != nil {
		return nil, err
	}

	if len(out) == 0 {
		return nil, nil
	}

	str := string(out)
	// Assuming formats: [], [foo], [foo bar]
	str = str[1 : len(str)-1]

	return strings.Split(str, " "), nil
}

func goDirs(files []string) stringSet {
	dirSet := make(stringSet)
	for _, file := range files {
		if strings.HasSuffix(file, ".go") {
			dirSet.add(filepath.Dir(file))
		}
	}

	return dirSet
}
